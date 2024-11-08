package testdata

//go:generate go run ../main.go -service Proto -target proto_gen.go
type Proto interface {
	AddClient(clientID string, client Client)
	RemoveClient(clientID string)
	Close()

	UpdateAndGet(map[int]int) (map[int]int, error)
}
type Client interface {
	Updated(map[int]int)
}
