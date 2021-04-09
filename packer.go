package play


type Packer interface {
	Read(c *Conn, data []byte) (*Request, []byte, error)
	Write(c *Conn, output Output) error
}