import io
p = 'test-data/cloud_mock.py'
s = io.open(p, encoding='utf-8').read()
i = s.find('AZURE_TOKEN =')
j = s.find('def serve(')
assert i > 0 and j > i
consts = '''AZURE_TOKEN = json.dumps({"access_token": "mock-token"})
AZURE_VMS = json.dumps({"value": [{
    "name": "azure-vm-01",
    "location": "eastus",
    "properties": {
        "vmId": "az-vm-001",
        "hardwareProfile": {"vmSize": "Standard_B1s"},
        "storageProfile": {"osDisk": {"osType": "Windows"}},
        "instanceView": {"statuses": [
            {"code": "ProvisioningState/succeeded"},
            {"code": "PowerState/running"}]},
        "networkProfile": {"networkInterfaces": [
            {"id": "/subscriptions/sub/providers/Microsoft.Network/networkInterfaces/nic1"}]},
    },
    "tags": {"Name": "azure-win-01"},
}]})
AZURE_NIC = json.dumps({"properties": {"ipConfigurations": [{"properties": {
    "privateIPAddress": "10.1.0.10",
    "publicIPAddress": {"id": "/subscriptions/sub/pip1"}}}]}})
AZURE_PIP = json.dumps({"properties": {"ipAddress": "20.1.1.1"}})

HW_TOKEN_HEADERS = {"X-Subject-Token": "mock-token"}
HW_PROJECTS = json.dumps({"projects": [{"id": "mockproj", "name": "mock"}]})
HW_SERVERS = json.dumps({"servers": [{
    "id": "hw-001", "name": "hw-linux-01", "status": "ACTIVE",
    "metadata": {"os_type": "Linux"},
    "flavor": {"id": "c7.small"},
    "addresses": {"mocknet": [
        {"version": "4", "addr": "192.168.1.10"},
        {"version": "4", "addr": "1.2.3.5", "OS-EXT-IPS:type": "floating"}]},
}]})

'''
s = s[:i] + consts + s[j:]
io.open(p, 'w', encoding='utf-8', newline='').write(s)
import json as J
for expr in (s[s.find('AZURE_TOKEN ='):s.find('\nAZURE_VMS')], s[s.find('AZURE_VMS ='):s.find('\nAZURE_NIC')], s[s.find('AZURE_NIC ='):s.find('\nAZURE_PIP')], s[s.find('AZURE_PIP ='):s.find('\nHW_TOKEN')], s[s.find('HW_SERVERS ='):s.find('\n\ndef serve')]):
    J.loads(expr.split('=', 1)[1].strip())
print('consts valid')
