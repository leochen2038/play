package play


type Packer interface {
	Read(c *Client, data []byte) (*Request, []byte, error)
	Write(c *Client, output Output) error
}