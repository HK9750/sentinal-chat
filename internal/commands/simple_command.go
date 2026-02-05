package commands

type SimpleCommand struct {
	Type                string
	Payload             any
	IdempotencyKeyValue string
	ValidateFunc        func(any) error
}

func (c SimpleCommand) CommandType() string {
	return c.Type
}

func (c SimpleCommand) Validate() error {
	if c.ValidateFunc != nil {
		return c.ValidateFunc(c.Payload)
	}
	return nil
}

func (c SimpleCommand) IdempotencyKey() string {
	return c.IdempotencyKeyValue
}
