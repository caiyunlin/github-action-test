az keyvault update --name "mykv0224" --resource-group "mine" --subscription "3c8fddff-d394-45bb-984a-19fb452e9cd2" --set tags.SecurityControl=Ignore

az keyvault update --name "mykv0224" --resource-group "mine" --subscription "3c8fddff-d394-45bb-984a-19fb452e9cd2" --public-network-access Enabled --default-action Allow --set tags.SecurityControl=Ignore

az keyvault show --name "mykv0224" --resource-group "mine" --subscription "3c8fddff-d394-45bb-984a-19fb452e9cd2" --query "{SecurityControl:tags.SecurityControl,publicNetworkAccess:properties.publicNetworkAccess,defaultAction:properties.networkAcls.defaultAction}" --output table

