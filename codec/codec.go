package codec

type Codec interface {
	Name() string
	Marshal(msg interface{}) ([]byte, error)
	UnMarshal(data []byte, msg interface{}) error
}
