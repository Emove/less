package transport

type Codec interface {
	Encode(res interface{}, writer Writer) (err error)
	Decode(reader Reader) (req interface{}, err error)
}
