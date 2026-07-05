package contract

type CanMarshal interface {
	Marshal() ([]byte, error)
}
