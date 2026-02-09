package commands

import "github.com/google/uuid"

type SimpleCommand struct {
	Type                string
	Payload             any
	IdempotencyKeyValue string
	ValidateFunc        func(any) error
	ActorIDValue        uuid.UUID
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

func (c SimpleCommand) ActorID() uuid.UUID {
	return c.ActorIDValue
}

func (c SimpleCommand) PayloadBytes() ([]byte, bool) {
	payload, ok := c.Payload.([]byte)
	return payload, ok
}
