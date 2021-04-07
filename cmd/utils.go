package cmd

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func boolAll(bools map[string]bool) bool {
	is_bool := true
	for _, elem := range bools {
		if !elem {
			is_bool = false
			break
		}
	}
	return is_bool
}
