package tools

// Map 实现一个函数式编程的map
func Map[C any, P any](slice []P, f func(P) C) []C {
	result := make([]C, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// Filter 实现一个函数式编程的filter
func Filter[C any](slice []C, f func(C) bool) []C {
	result := make([]C, 0)
	for _, v := range slice {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce 实现一个函数式编程的reduce
// T 没有使用 any 关键字，表示它可以是任何类型，但是在使用时，需要在调用函数时指定具体的类型。例如，您可以调用 Reduce 函数时指定 T 为 int、string 或其他类型。
// M 使用了 any 关键字，表示它可以是任何类型，并且在函数内部可以根据需要使用它。在这种情况下，编译器会根据函数调用时的上下文推断 M 的实际类型。（也可以指定）
func Reduce[T, M any](s []T, initValue M, f func(M, T) M) M {
	acc := initValue
	for _, v := range s {
		acc = f(acc, v)
	}
	return acc
}

// reduce example bool求和
//needArchiveNum := tools.Reduce(needArchiveList, 0, func(acc int, current bool) int {
//	if current {
//		return acc + 1
//	}
//	return acc
//})
