package tools

// Map 实现一个函数式编程的map
func Map[C any, P any](slice []P, f func(P) C) []C {
	result := make([]C, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}
