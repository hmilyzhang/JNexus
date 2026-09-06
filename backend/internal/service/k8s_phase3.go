// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// K8S 三期：StatefulSet 伸缩 / YAML 更新 / Pod 日志实时跟随

// ScaleStatefulSet 调整 StatefulSet 副本数（scale 子资源）
func (k *K8sAPI) ScaleStatefulSet(namespace, name string, replicas int) error {
	body, _ := json.Marshal(map[string]any{
		"apiVersion": "apps/v1", "kind": "Scale",
		"metadata": map[string]any{"name": name, "namespace": namespace},
		"spec":     map[string]any{"replicas": replicas},
	})
	return k.do("PATCH", "/apis/apps/v1/namespaces/"+namespace+"/statefulsets/"+name+"/scale", body, nil)
}

// UpdateResourceYAML 用编辑后的 YAML 替换资源（整对象 PUT）
func (k *K8sAPI) UpdateResourceYAML(kind, namespace, name, yamlText string) error {
	tpl, ok := k8sYAMLPaths[kind]
	if !ok {
		return fmt.Errorf("不支持的资源类型: %s", kind)
	}
	var path string
	if kind == "node" || kind == "pv" || kind == "storageclass" {
		path = fmt.Sprintf(tpl, name)
	} else {
		if namespace == "" {
			return fmt.Errorf("namespace 必填")
		}
		path = fmt.Sprintf(tpl, namespace, name)
	}
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &obj); err != nil {
		return fmt.Errorf("YAML 解析失败: %w", err)
	}
	if obj == nil {
		return fmt.Errorf("YAML 内容为空")
	}
	// 元数据一致性校验，防止把 A 资源的内容提交到 B
	if md, ok := obj["metadata"].(map[string]any); ok {
		if n, _ := md["name"].(string); n != "" && n != name {
			return fmt.Errorf("YAML 中 metadata.name(%s) 与目标资源(%s)不一致", n, name)
		}
		if ns, _ := md["namespace"].(string); namespace != "" && ns != "" && ns != namespace {
			return fmt.Errorf("YAML 中 metadata.namespace(%s) 与目标命名空间(%s)不一致", ns, namespace)
		}
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return k.do("PUT", path, body, nil)
}

// FollowPodLog 打开 follow=true 的日志流，返回响应体（调用方负责 close 与 cancel）
func (k *K8sAPI) FollowPodLog(ctx context.Context, namespace, name, container string, tail int) (io.ReadCloser, error) {
	path := "/api/v1/namespaces/" + namespace + "/pods/" + name + "/log?follow=true"
	if container != "" {
		path += "&container=" + container
	}
	if tail > 0 {
		path += "&tailLines=" + strconv.Itoa(tail)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", k.Server+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := k.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, fmt.Errorf("K8S API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return resp.Body, nil
}
