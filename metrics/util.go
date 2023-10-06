package metrics

func bool2float64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
