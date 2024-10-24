package testdata

//go:generate go run ../main.go -service Proto -target proto_gen.go
type Proto interface {
	UpdateAndGet(map[int]int) (map[int]int, error)
}
