package rbac

func IntToBinary(n, lenght int) []int {
	binary := make([]int, lenght)
	for i := 0; i < lenght; i++ {
		binary[i] = n % 2
		n /= 2
	}
	return binary
}
