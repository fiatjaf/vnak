package main

import (
	"context"
	"fmt"

	"fiatjaf.com/nostr"
)

func signAuthEvent(ctx context.Context, evt *nostr.Event) error {
	if currentKeyer != nil {
		err := currentKeyer.SignEvent(ctx, evt)
		if err != nil {
			return fmt.Errorf("failed to sign auth event: %w", err)
		}
		return nil
	}
	return fmt.Errorf("can't sign auth event, no key")
}
