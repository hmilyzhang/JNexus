// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// CSP adapters for cloud asset discovery (AWS / Azure / Huawei Cloud).
// Each adapter lists VM instances for the configured regions and normalizes
// them into CloudInstance records for the asset sync. An optional endpoint
// override in the credential JSON redirects API calls (private clouds, tests).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconf "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	ecsregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/region"
	"jnexus/internal/pkg"
)

// CloudInstance is the normalized inventory record from any provider
type CloudInstance struct {
	InstanceID   string            `json:"instance_id"`
	Name         string            `json:"name"`
	PrivateIP    string            `json:"private_ip"`
	PublicIP     string            `json:"public_ip"`
	OSType       string            `json:"os_type"` // linux / windows
	State        string            `json:"state"`   // running / stopped / other
	Region       string            `json:"region"`
	InstanceType string            `json:"instance_type"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// ---- provider credentials (decrypted form) ----

type AwsCred struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Endpoint  string `json:"endpoint"`
}

type AzureCred struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
	Endpoint       string `json:"endpoint"` // ARM base, default https://management.azure.com
}

type HuaweiCred struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Endpoint  string `json:"endpoint"`
	ProjectID string `json:"project_id"`
}

func decryptCredJSON(enc string, into any) error {
	plain, err := pkg.Decrypt(enc)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(plain), into)
}

// ListCloudInstances discovers instances for a provider (regions = CSV field)
func ListCloudInstances(provider, credEnc, regionsCSV string) ([]CloudInstance, error) {
	regions := splitCSV(regionsCSV)
	switch strings.ToLower(provider) {
	case "aws":
		var cred AwsCred
		if err := decryptCredJSON(credEnc, &cred); err != nil {
			return nil, err
		}
		return awsListInstances(cred, regions)
	case "azure":
		var cred AzureCred
		if err := decryptCredJSON(credEnc, &cred); err != nil {
			return nil, err
		}
		return azureListInstances(cred, regions)
	case "huawei":
		var cred HuaweiCred
		if err := decryptCredJSON(credEnc, &cred); err != nil {
			return nil, err
		}
		return huaweiListInstances(cred, regions)
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func splitCSV(csvStr string) []string {
	out := []string{}
	for _, p := range strings.Split(csvStr, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---- AWS ----

func awsListInstances(cred AwsCred, regions []string) ([]CloudInstance, error) {
	if len(regions) == 0 {
		regions = []string{"us-east-1"}
	}
	var (
		mu   sync.Mutex
		out  []CloudInstance
		errs []string
		wg   sync.WaitGroup
	)
	for _, regionName := range regions {
		wg.Add(1)
		go func(regionName string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cfg, err := awsconf.LoadDefaultConfig(ctx,
				awsconf.WithRegion(regionName),
				awsconf.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cred.AccessKey, cred.SecretKey, "")),
			)
			if err != nil {
				mu.Lock()
				errs = append(errs, regionName+": "+err.Error())
				mu.Unlock()
				return
			}
			client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
				if cred.Endpoint != "" {
					o.BaseEndpoint = aws.String(cred.Endpoint)
				}
			})
			paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				if err != nil {
					mu.Lock()
					errs = append(errs, regionName+": "+err.Error())
					mu.Unlock()
					return
				}
				mu.Lock()
				for _, res := range page.Reservations {
					for _, inst := range res.Instances {
						ci := CloudInstance{
							InstanceID:   aws.ToString(inst.InstanceId),
							PrivateIP:    aws.ToString(inst.PrivateIpAddress),
							PublicIP:     aws.ToString(inst.PublicIpAddress),
							InstanceType: string(inst.InstanceType),
							Region:       regionName,
							Tags:         map[string]string{},
						}
						if string(inst.Platform) == "windows" {
							ci.OSType = "windows"
						} else {
							ci.OSType = "linux"
						}
						if inst.State != nil {
							ci.State = string(inst.State.Name)
						}
						for _, tag := range inst.Tags {
							ci.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
						}
						ci.Name = ci.Tags["Name"]
						if ci.Name == "" {
							ci.Name = ci.InstanceID
						}
						out = append(out, ci)
					}
				}
				mu.Unlock()
			}
		}(regionName)
	}
	wg.Wait()
	if len(errs) > 0 {
		return out, fmt.Errorf("AWS 拉取失败: %s", strings.Join(errs, "; "))
	}
	return out, nil
}

// ---- Azure (REST + client-credentials OAuth) ----

type azureTokenResp struct {
	AccessToken string `json:"access_token"`
}

func azureREST(ctx context.Context, httpClient *http.Client, method, url, token string, form url.Values) (statusCode int, body []byte) {
	var req *http.Request
	if form != nil {
		req, _ = http.NewRequestWithContext(ctx, method, url, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, _ = http.NewRequestWithContext(ctx, method, url, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, b
}

func azureToken(ctx context.Context, httpClient *http.Client, cred AzureCred) (string, string, error) {
	base := cred.Endpoint
	if base == "" {
		base = "https://management.azure.com"
	}
	armBase := strings.TrimSuffix(base, "/")
	// token host derived from the ARM override for testability; default login endpoint otherwise
	tokenURL := "https://login.microsoftonline.com/" + cred.TenantID + "/oauth2/v2.0/token"
	if armBase != "https://management.azure.com" {
		tokenURL = armBase + "/{tenant}/oauth2/v2.0/token"
		tokenURL = strings.ReplaceAll(tokenURL, "{tenant}", cred.TenantID)
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cred.ClientID)
	form.Set("client_secret", cred.ClientSecret)
	form.Set("scope", "https://management.azure.com/.default")
	st, body := azureREST(ctx, httpClient, http.MethodPost, tokenURL, "", form)
	if st != 200 {
		return "", "", fmt.Errorf("Azure token %d: %s", st, string(body))
	}
	var t azureTokenResp
	if err := json.Unmarshal(body, &t); err != nil {
		return "", "", err
	}
	if t.AccessToken == "" {
		return "", "", fmt.Errorf("Azure token 为空")
	}
	return t.AccessToken, armBase, nil
}

func azureListInstances(cred AzureCred, regions []string) ([]CloudInstance, error) {
	if len(regions) == 0 {
		regions = []string{"eastus"}
	}
	_ = regions // all-list returns every region; region filter applied client-side
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: OutboundTLS()}}
	token, armBase, err := azureToken(ctx, httpClient, cred)
	if err != nil {
		return nil, err
	}

	nicCache := map[string]string{} // nic id -> private ip
	pipCache := map[string]string{} // public ip resource id -> address
	getJSON := func(path string) (map[string]any, int, error) {
		st, body := azureREST(ctx, httpClient, http.MethodGet, armBase+path, token, nil)
		if st != 200 {
			return nil, st, fmt.Errorf("Azure GET %s -> %d: %s", path, st, string(body))
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, st, err
		}
		return m, st, nil
	}
	getStr := func(m map[string]any, keys ...string) string {
		cur := m
		for i, k := range keys {
			v, ok := cur[k]
			if !ok || v == nil {
				return ""
			}
			if i == len(keys)-1 {
				s, _ := v.(string)
				return s
			}
			cur, ok = v.(map[string]any)
			if !ok {
				return ""
			}
		}
		return ""
	}
	resolveIP := func(nicID string) (string, string) {
		if priv, ok := nicCache[nicID]; ok {
			return priv, pipCache[nicID]
		}
		priv, pip := "", ""
		m, _, err := getJSON("/" + nicID + "?api-version=2021-07-01")
		if err == nil {
			props, _ := m["properties"].(map[string]any)
			if cfgs, ok := props["ipConfigurations"].([]any); ok && len(cfgs) > 0 {
				if cfg, ok := cfgs[0].(map[string]any); ok {
					if cprops, ok := cfg["properties"].(map[string]any); ok {
						priv, _ = cprops["privateIPAddress"].(string)
						if pipRef, ok := cprops["publicIPAddress"].(map[string]any); ok {
							if pid, ok := pipRef["id"].(string); ok {
								pm, _, perr := getJSON("/" + pid + "?api-version=2021-07-01")
								if perr == nil {
									pprops, _ := pm["properties"].(map[string]any)
									pip, _ = pprops["ipAddress"].(string)
									pipCache[pid] = pip
								}
							}
						}
					}
				}
			}
		}
		nicCache[nicID], pipCache[nicID] = priv, pip
		return priv, pip
	}

	out := []CloudInstance{}
	path := "/subscriptions/" + cred.SubscriptionID + "/providers/Microsoft.Compute/virtualMachines/all?api-version=2021-07-01"
	for page := 0; page < 20; page++ {
		m, st, err := getJSON(path)
		if err != nil {
			return out, err
		}
		_ = st
		vals, _ := m["value"].([]any)
		for _, v := range vals {
			vm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			props, _ := vm["properties"].(map[string]any)
			ci := CloudInstance{Tags: map[string]string{}}
			ci.Name, _ = vm["name"].(string)
			ci.InstanceID, _ = props["vmId"].(string)
			if ci.InstanceID == "" {
				ci.InstanceID = ci.Name
			}
			ci.InstanceType = getStr(props, "hardwareProfile", "vmSize")
			ci.OSType = strings.ToLower(getStr(props, "storageProfile", "osDisk", "osType"))
			// power state
			stStr := "running"
			if iv, ok := props["instanceView"].(map[string]any); ok {
				if statuses, ok := iv["statuses"].([]any); ok {
					for _, stv := range statuses {
						if sm, ok := stv.(map[string]any); ok {
							if code, _ := sm["code"].(string); strings.HasPrefix(code, "PowerState/") {
								stStr = strings.TrimPrefix(code, "PowerState/")
							}
						}
					}
				}
			}
			if stStr == "running" {
				ci.State = "running"
			} else if strings.Contains(stStr, "dealloc") || stStr == "stopped" {
				ci.State = "stopped"
			} else {
				ci.State = stStr
			}
			// tags
			if tags, ok := vm["tags"].(map[string]any); ok {
				for tk, tv := range tags {
					ci.Tags[tk], _ = tv.(string)
				}
			}
			ci.Name = orDefault(ci.Tags["Name"], ci.Name)
			// location -> region filter
			loc, _ := vm["location"].(string)
			if len(regions) > 0 {
				match := false
				for _, want := range regions {
					if strings.EqualFold(strings.TrimSpace(loc), strings.TrimSpace(want)) {
						match = true
					}
				}
				if !match {
					continue
				}
			}
			ci.Region = loc
			// primary NIC private/public IP
			if np, ok := props["networkProfile"].(map[string]any); ok {
				if nics, ok := np["networkInterfaces"].([]any); ok && len(nics) > 0 {
					if nic0, ok := nics[0].(map[string]any); ok {
						if id, ok := nic0["id"].(string); ok {
							ci.PrivateIP, ci.PublicIP = resolveIP(id)
						}
					}
				}
			}
			out = append(out, ci)
		}
		nextLink, _ := m["nextLink"].(string)
		if nextLink == "" {
			break
		}
		path = "/" + strings.TrimPrefix(nextLink, armBase)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	return out, nil
}

func orDefault(v, d string) string {
	if v != "" {
		return v
	}
	return d
}

// ---- Huawei Cloud ----

func huaweiListInstances(cred HuaweiCred, regions []string) (out []CloudInstance, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			out, err = nil, fmt.Errorf("华为云 SDK 异常: %v（请检查 AK/SK、project_id 与区域配置）", rec)
		}
	}()
	if len(regions) == 0 {
		regions = []string{"cn-north-4"}
	}
	for _, regionName := range regions {
		var rg *region.Region
		if cred.Endpoint != "" {
			// custom/private endpoint: region id is a free-form label
			rg = region.NewRegion(regionName, cred.Endpoint)
		} else {
			// official region table (panics on unknown ids - convert to error)
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						err = fmt.Errorf("华为云区域 %q 不受支持: %v", regionName, rec)
					}
				}()
				rg = ecsregion.ValueOf(regionName)
			}()
			if rg == nil || err != nil {
				return out, err
			}
		}
		builder := basic.NewCredentialsBuilder().WithAk(cred.AccessKey).WithSk(cred.SecretKey)
		if cred.ProjectID != "" {
			builder = builder.WithProjectId(cred.ProjectID)
		}
		if cred.Endpoint != "" {
			// private-cloud deployments: route IAM (project resolution) to the same endpoint
			builder = builder.WithIamEndpointOverride(cred.Endpoint)
		}
		creds := builder.Build()
		client := ecs.NewEcsClient(ecs.EcsClientBuilder().WithRegion(rg).WithCredential(creds).Build())

		// paged fetch (limit + item offset) so accounts with many instances import fully
		var pageSize int32 = 500
		for offset := int32(0); ; offset += pageSize {
			lim, off := pageSize, offset
			resp, err := client.ListServersDetails(&ecsmodel.ListServersDetailsRequest{Limit: &lim, Offset: &off})
			if err != nil {
				return out, fmt.Errorf("华为云 %s: %w", regionName, err)
			}
			if resp.Servers == nil || len(*resp.Servers) == 0 {
				break
			}
			for _, srv := range *resp.Servers {
				ci := CloudInstance{Region: regionName, Tags: map[string]string{}}
				ci.InstanceID = srv.Id
				ci.Name = srv.Name
				switch srv.Status {
				case "ACTIVE":
					ci.State = "running"
				case "SHUTOFF":
					ci.State = "stopped"
				default:
					ci.State = srv.Status
				}
				for k, v := range srv.Metadata {
					ci.Tags[k] = v
					if k == "os_type" {
						ci.OSType = strings.ToLower(v)
					}
				}
				if srv.Flavor != nil {
					ci.InstanceType = srv.Flavor.Id
				}
				enum := ecsmodel.GetServerAddressOSEXTIPStypeEnum()
				for _, addrs := range srv.Addresses {
					for _, a := range addrs {
						if a.Version == "6" {
							continue
						}
						if a.OSEXTIPStype != nil && *a.OSEXTIPStype == enum.FLOATING {
							if ci.PublicIP == "" {
								ci.PublicIP = a.Addr
							}
						} else if ci.PrivateIP == "" {
							ci.PrivateIP = a.Addr
						}
					}
				}
				if ci.Name == "" {
					ci.Name = ci.InstanceID
				}
				out = append(out, ci)
			}
			if len(*resp.Servers) < int(pageSize) {
				break
			}
		}
	}
	return out, nil
}
