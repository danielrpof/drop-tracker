package deezer

import (
	"context"
	"errors"
)

// ArtistByID is a RED-phase placeholder -- not yet implemented. See
// artist_test.go for the behavior this must satisfy.
func (c *Client) ArtistByID(ctx context.Context, artistID string) (Artist, error) {
	return Artist{}, errors.New("deezer: ArtistByID not yet implemented")
}
