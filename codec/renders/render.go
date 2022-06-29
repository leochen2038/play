package renders

type Render interface {
	Name() string
	Render(data map[string]interface{}) ([]byte, error)
}
