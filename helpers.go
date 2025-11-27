package main

import (
	"context"
	"fmt"
	"strings"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/keyer"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip46"
)

func handleSecretKeyOrBunker(sec string) (nostr.SecretKey, nostr.Keyer, error) {
	if strings.HasPrefix(sec, "bunker://") {
		// it's a bunker
		bunkerURL := sec
		clientKey := nostr.Generate()
		ctx := context.Background()

		bunker, err := nip46.ConnectBunker(ctx, clientKey, bunkerURL, nil, func(s string) {})
		if err != nil {
			return nostr.SecretKey{}, nil, fmt.Errorf("failed to connect to %s: %w", bunkerURL, err)
		}

		return nostr.SecretKey{}, keyer.NewBunkerSignerFromBunkerClient(bunker), err
	}

	if prefix, ski, err := nip19.Decode(sec); err == nil && prefix == "nsec" {
		sk := ski.(nostr.SecretKey)
		return sk, keyer.NewPlainKeySigner(sk), nil
	}

	sk, err := nostr.SecretKeyFromHex(sec)
	if err != nil {
		return nostr.SecretKey{}, nil, fmt.Errorf("invalid secret key: %w", err)
	}

	return sk, keyer.NewPlainKeySigner(sk), nil
}

func decodeTagValue(value string) string {
	if strings.HasPrefix(value, "npub1") || strings.HasPrefix(value, "nevent1") || strings.HasPrefix(value, "note1") || strings.HasPrefix(value, "nprofile1") || strings.HasPrefix(value, "naddr1") {
		if ptr, err := nip19.ToPointer(value); err == nil {
			return ptr.AsTagReference()
		}
	}
	return value
}
