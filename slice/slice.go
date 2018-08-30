package slice

func Delete(slice []string, str string) []string {
	var result []string

	for _, v := range slice {
		if v != str {
			result = append(result, v)
		}
	}

	return result
}
