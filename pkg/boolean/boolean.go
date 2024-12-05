package boolean

func IsTrue(b *bool) bool {
	return b != nil && *b
}
