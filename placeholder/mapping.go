package placeholder

import (
	_ "embed"
	"encoding/json"
	"log"
	"strings"

	"github.com/ms-henglu/aztfpreflight/account"
)

//go:embed mappings.mini.json
var m string

var mapping map[string]map[string]string
var pathPlaceholderMap map[string]interface{}

func init() {
	type Mapping struct {
		ResourceType         string `json:"resourceType"`
		ExampleConfiguration string `json:"exampleConfiguration,omitempty"`
		IdPattern            string `json:"idPattern"`
	}

	var array []Mapping
	err := json.Unmarshal([]byte(m), &array)
	if err != nil {
		panic(err)
	}

	mapping = make(map[string]map[string]string)
	for _, item := range array {
		parts := strings.Split(item.IdPattern, "/")
		out := ""
		for index, part := range parts {
			switch part {
			case "subscriptions":
				out += "/subscriptions/00000000-0000-0000-0000-000000000000"
			case "resourceGroups":
				out += "/resourceGroups/myResourceGroup"
			case "providers":
				out += "/providers"
			case "":
			default:
				if index > 0 && parts[index-1] == "providers" {
					out += "/" + part
				} else {
					out += "/" + part + "/" + part + "Name"
				}
			}
		}
		if _, ok := mapping[item.ResourceType]; !ok {
			mapping[item.ResourceType] = make(map[string]string)
		}
		mapping[item.ResourceType]["id"] = out
	}

	// adding hardcoded mappings
	hardcodedMapping := make(map[string]map[string]string)
	hardcodedMapping["azurerm_subscription"] = map[string]string{
		"id": "/subscriptions/00000000-0000-0000-0000-000000000000",
	}
	hardcodedMapping["azurerm_storage_container"] = map[string]string{
		"resource_manager_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Storage/storageAccounts/myStorageAccount/blobServices/default/containers/myContainer",
	}
	hardcodedMapping["azurerm_lb"] = map[string]string{
		"frontend_ip_configuration[0].id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/loadBalancers/myLB/frontendIPConfigurations/myFrontendIPConfiguration",
		"id":                              "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/loadBalancers/myLB",
	}
	hardcodedMapping["azurerm_vpn_site"] = map[string]string{
		"link[0].id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/vpnSites/myVpnSite/links/myLink",
	}
	hardcodedMapping["azurerm_storage_account"] = map[string]string{
		"primary_access_key":             "ZmFrZV9hY2Nlc3Nfa2V5",
		"primary_blob_endpoint":          "https://myStorageAccount.blob.core.windows.net/",
		"primary_blob_connection_string": "DefaultEndpointsProtocol=https;AccountName=myStorageAccount;AccountKey=ZmFrZV9hY2Nlc3Nfa2V5;EndpointSuffix=core.windows.net",
	}
	hardcodedMapping["azurerm_sentinel_log_analytics_workspace_onboarding"] = map[string]string{
		"workspace_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.OperationalInsights/workspaces/myWorkspace",
	}
	hardcodedMapping["azurerm_application_insights"] = map[string]string{
		"instrumentation_key": "00000000-0000-0000-0000-000000000000",
	}
	hardcodedMapping["azurerm_public_ip"] = map[string]string{
		"ip_address": "123.123.123.123",
	}
	hardcodedMapping["azurerm_databricks_virtual_network_peering"] = map[string]string{
		"virtual_network_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVnet",
	}
	hardcodedMapping["azurerm_eventhub_namespace"] = map[string]string{
		"default_primary_key": "ZmFrZV9hY2Nlc3Nfa2V5",
	}
	hardcodedMapping["azurerm_app_service"] = map[string]string{
		"default_site_hostname": "myAppService.azurewebsites.net",
	}
	hardcodedMapping["azurerm_app_service"] = map[string]string{
		"custom_domain_verification_id": "myCustomDomainVerificationId",
	}
	hardcodedMapping["azurerm_storage_data_lake_gen2_filesystem"] = map[string]string{
		"id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Storage/storageAccounts/myStorageAccount/filesystems/myFileSystem",
	}
	hardcodedMapping["azurerm_sentinel_alert_rule_anomaly"] = map[string]string{
		"id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.OperationalInsights/workspaces/myWorkspace/providers/Microsoft.SecurityInsights/securityMLAnalyticsSettings/mySecurityMLAnalyticsSetting",
	}
	hardcodedMapping["azurerm_resource_group_policy_assignment"] = map[string]string{
		"resource_group_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup",
	}
	hardcodedMapping["azurerm_virtual_desktop_host_pool"] = map[string]string{
		"id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.DesktopVirtualization/hostPools/myHostPool",
	}
	hardcodedMapping["azurerm_managed_api"] = map[string]string{
		"id": "/subscriptions/12345678-1234-9876-4563-123456789012/providers/Microsoft.Web/locations/locationName/managedApis/managedApiName",
	}
	hardcodedMapping["azurerm_policy_definition"] = map[string]string{
		"id": "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Authorization/policyDefinitions/myPolicyDefinition",
	}
	hardcodedMapping["azurerm_backup_container_storage_account"] = map[string]string{
		"storage_account_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Storage/storageAccounts/myStorageAccount",
	}
	hardcodedMapping["azurerm_vmware_private_cloud"] = map[string]string{
		"circuit[0].express_route_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/expressRouteCircuits/myExpressRouteCircuit",
	}
	hardcodedMapping["azurerm_eventgrid_topic"] = map[string]string{
		"endpoint":             "https://myeventgridtopic.westus-1.eventgrid.azure.net/api/events",
		"primary_access_key":   "ZmFrZV9hY2Nlc3Nfa2V5",
		"secondary_access_key": "ZmFrZV9hY2Nlc3Nfa2V5",
	}
	hardcodedMapping["azurerm_user_assigned_identity"] = map[string]string{
		"principal_id": "00000000-0000-0000-0000-000000000000",
	}
	hardcodedMapping["azurerm_storage_blob"] = map[string]string{
		"id": "https://myStorageAccount.blob.core.windows.net/myContainer/myBlob",
	}
	hardcodedMapping["azurerm_chaos_studio_target"] = map[string]string{
		"id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Chaos/targets/myTarget",
	}
	hardcodedMapping["azurerm_key_vault_key"] = map[string]string{
		"id": "https://myKeyVault.vault.azure.net/keys/myKey/00000000000000000000000000000000",
	}

	// merge hardcoded mappings
	for key, value := range hardcodedMapping {
		if _, ok := mapping[key]; !ok {
			mapping[key] = make(map[string]string)
		}
		for k, v := range value {
			mapping[key][k] = v
		}
	}

	for key, _ := range mapping {
		mapping[key]["identity[0].principal_id"] = "00000000-0000-0000-0000-000000000000"
		mapping[key]["identity[0].tenant_id"] = "00000000-0000-0000-0000-000000000000"
	}

	pathPlaceholderMap = map[string]interface{}{
		"azurerm_virtual_network_gateway.ip_configuration.0.subnet_id":                                           "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVnet/subnets/GatewaySubnet",
		"azurerm_firewall.ip_configuration.0.subnet_id":                                                          "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVnet/subnets/AzureFirewallSubnet",
		"azurerm_network_interface_application_gateway_backend_address_pool_association.backend_address_pool_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/applicationGateways/myAppGateway/backendAddressPools/myBackendAddressPool",
		"azurerm_spring_cloud_app.addon_json":                                                                    "{}",
		"azurerm_dev_center_dev_box_definition.image_reference_id":                                               "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/example-resource-group/providers/Microsoft.DevCenter/devCenters/devCenterName/galleries/galleryName/images/imageName",
		"azurerm_sentinel_alert_rule_machine_learning_behavior_analytics.alert_rule_template_guid":               "00000000-0000-0000-0000-000000000000",
		"azurerm_frontdoor_custom_https_configuration.frontend_endpoint_id":                                      "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/frontDoors/myFrontDoor/frontendEndpoints/myFrontendEndpoint",
		"azurerm_bastion_host.ip_configuration.0.subnet_id":                                                      "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVnet/subnets/AzureBastionSubnet",
		"azurerm_sentinel_alert_rule_threat_intelligence.alert_rule_template_guid":                               "00000000-0000-0000-0000-000000000000",
		"azurerm_sentinel_alert_rule_fusion.alert_rule_template_guid":                                            "00000000-0000-0000-0000-000000000000",
		"azurerm_vmware_netapp_volume_attachment.vmware_cluster_id":                                              "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.AVS/privateClouds/myPrivateCloud/clusters/myCluster",
		"azurerm_vpn_gateway_connection.vpn_link.0.vpn_site_link_id":                                             "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/vpnSites/myVpnSite/vpnSiteLinks/myVpnSiteLink",
	}

	// replace empty subscription id with actual subscription id
	resourceManagerAccount := account.DefaultSharedAccount()
	subscriptionId := resourceManagerAccount.GetSubscriptionId()
	if subscriptionId == "" {
		log.Printf("[WARN] No subscription ID found, please set default subscription ID in az cli or pass it in environment variable AZURE_SUBSCRIPTION_ID")
		return
	}
	for key, value := range pathPlaceholderMap {
		if str, ok := value.(string); ok {
			pathPlaceholderMap[key] = strings.ReplaceAll(str, "/subscriptions/00000000-0000-0000-0000-000000000000", "/subscriptions/"+subscriptionId)
		}
	}
	for key, value := range mapping {
		for k, v := range value {
			mapping[key][k] = strings.ReplaceAll(v, "/subscriptions/00000000-0000-0000-0000-000000000000", "/subscriptions/"+subscriptionId)
		}
	}
}
