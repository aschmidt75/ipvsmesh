package model

// GetServicesAddresses retrieves all service addresses from IPVSModelStruct
func (m *IPVSModelStruct) GetServicesAddresses() []string {
	servicesRaw, ex := (*m)["services"]
	if !ex {
		// no services there.
		return make([]string, 0)
	}

	services := servicesRaw.([]interface{})

	res := make([]string, len(services))
	idx := 0
	for _, serviceRaw := range services {
		service := serviceRaw.(map[string]interface{})

		res[idx] = service["address"].(string)
		idx++
	}

	return res
}
