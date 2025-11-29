package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/keyer"
	"fiatjaf.com/nostr/nip05"
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

func parsePubKey(value string) (nostr.PubKey, error) {
	// try nip05 first
	if nip05.IsValidIdentifier(value) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		pp, err := nip05.QueryIdentifier(ctx, value)
		cancel()
		if err == nil {
			return pp.PublicKey, nil
		}
		// if nip05 fails, fall through to try as pubkey
	}

	pk, err := nostr.PubKeyFromHex(value)
	if err == nil {
		return pk, nil
	}

	if prefix, decoded, err := nip19.Decode(value); err == nil {
		switch prefix {
		case "npub":
			if pk, ok := decoded.(nostr.PubKey); ok {
				return pk, nil
			}
		case "nprofile":
			if profile, ok := decoded.(nostr.ProfilePointer); ok {
				return profile.PublicKey, nil
			}
		}
	}

	return nostr.PubKey{}, fmt.Errorf("invalid pubkey (\"%s\"): expected hex, npub, or nprofile", value)
}

func parseEventID(value string) (nostr.ID, error) {
	id, err := nostr.IDFromHex(value)
	if err == nil {
		return id, nil
	}

	if prefix, decoded, err := nip19.Decode(value); err == nil {
		switch prefix {
		case "note":
			if id, ok := decoded.(nostr.ID); ok {
				return id, nil
			}
		case "nevent":
			if event, ok := decoded.(nostr.EventPointer); ok {
				return event.ID, nil
			}
		}
	}

	return nostr.ID{}, fmt.Errorf("invalid event id (\"%s\"): expected hex, note, or nevent", value)
}

func decodeTagValue(value string) string {
	if strings.HasPrefix(value, "npub1") || strings.HasPrefix(value, "nevent1") || strings.HasPrefix(value, "note1") || strings.HasPrefix(value, "nprofile1") || strings.HasPrefix(value, "naddr1") {
		if ptr, err := nip19.ToPointer(value); err == nil {
			return ptr.AsTagReference()
		}
	}
	return value
}

func niceRelayURL(url string) string {
	return strings.SplitN(nostr.NormalizeURL(url), "/", 3)[2]
}

func niceRelayURLs(urls []string) []string {
	nices := make([]string, len(urls))
	for i, url := range urls {
		nices[i] = strings.SplitN(nostr.NormalizeURL(url), "/", 3)[2]
	}
	return nices
}
