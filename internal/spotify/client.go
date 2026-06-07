// Package spotify wraps the Spotify Web API (via zmb3/spotify) behind small
// domain types, so the UI never depends on the library directly.
//
// The Web API is used purely for control and metadata — actual audio is played
// by the librespot device this client targets.
package spotify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	zspotify "github.com/zmb3/spotify/v2"
)

// Track is a single playable track.
type Track struct {
	ID       zspotify.ID
	URI      zspotify.URI
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	ImageURL string
}

// Playlist is an entry in the library sidebar.
type Playlist struct {
	ID    zspotify.ID
	URI   zspotify.URI
	Name  string
	Count int
}

// Device is a Spotify Connect playback device.
type Device struct {
	ID     string
	Name   string
	Active bool
	Volume int
}

// PlayerState is the current playback snapshot.
type PlayerState struct {
	Track      *Track
	Playing    bool
	Progress   time.Duration
	Volume     int
	Shuffle    bool
	Repeat     string // "off" | "context" | "track"
	DeviceID   string
	DeviceName string
}

// Client is the AudioPulse Spotify wrapper.
type Client struct {
	api *zspotify.Client
}

// New builds a Client from an authorized HTTP client (see internal/auth).
func New(httpClient *http.Client) *Client {
	return &Client{api: zspotify.New(httpClient, zspotify.WithRetry(true))}
}

// Me returns the current user's display name, doubling as a token validity check.
func (c *Client) Me(ctx context.Context) (string, error) {
	u, err := c.api.CurrentUser(ctx)
	if err != nil {
		return "", err
	}
	if u.DisplayName != "" {
		return u.DisplayName, nil
	}
	return u.ID, nil
}

// --- library -----------------------------------------------------------------

// Playlists returns the user's playlists.
func (c *Client) Playlists(ctx context.Context) ([]Playlist, error) {
	page, err := c.api.CurrentUsersPlaylists(ctx, zspotify.Limit(50))
	if err != nil {
		return nil, err
	}
	out := make([]Playlist, 0, len(page.Playlists))
	for _, p := range page.Playlists {
		out = append(out, Playlist{
			ID:    p.ID,
			URI:   p.URI,
			Name:  p.Name,
			Count: int(p.Tracks.Total),
		})
	}
	return out, nil
}

// PlaylistTracks returns the tracks of a playlist.
func (c *Client) PlaylistTracks(ctx context.Context, id zspotify.ID) ([]Track, error) {
	page, err := c.api.GetPlaylistItems(ctx, id, zspotify.Limit(100))
	if err != nil {
		return nil, err
	}
	out := make([]Track, 0, len(page.Items))
	for _, it := range page.Items {
		if it.Track.Track != nil {
			out = append(out, toTrack(it.Track.Track))
		}
	}
	return out, nil
}

// LikedSongs returns the user's saved tracks.
func (c *Client) LikedSongs(ctx context.Context) ([]Track, error) {
	page, err := c.api.CurrentUsersTracks(ctx, zspotify.Limit(50))
	if err != nil {
		return nil, err
	}
	out := make([]Track, 0, len(page.Tracks))
	for i := range page.Tracks {
		out = append(out, toTrack(&page.Tracks[i].FullTrack))
	}
	return out, nil
}

// RecentlyPlayed returns recently played tracks.
func (c *Client) RecentlyPlayed(ctx context.Context) ([]Track, error) {
	items, err := c.api.PlayerRecentlyPlayed(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Track, 0, len(items))
	for i := range items {
		out = append(out, simpleToTrack(&items[i].Track))
	}
	return out, nil
}

// Search returns tracks matching a query.
func (c *Client) Search(ctx context.Context, query string) ([]Track, error) {
	res, err := c.api.Search(ctx, query, zspotify.SearchTypeTrack, zspotify.Limit(50))
	if err != nil {
		return nil, err
	}
	var out []Track
	if res.Tracks != nil {
		for i := range res.Tracks.Tracks {
			out = append(out, toTrack(&res.Tracks.Tracks[i]))
		}
	}
	return out, nil
}

// Queue returns the upcoming tracks in the play queue.
func (c *Client) Queue(ctx context.Context) ([]Track, error) {
	q, err := c.api.GetQueue(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Track, 0, len(q.Items))
	for i := range q.Items {
		out = append(out, toTrack(&q.Items[i]))
	}
	return out, nil
}

// --- devices & playback ------------------------------------------------------

// Devices lists available Connect devices.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	ds, err := c.api.PlayerDevices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(ds))
	for _, d := range ds {
		out = append(out, Device{
			ID:     string(d.ID),
			Name:   d.Name,
			Active: d.Active,
			Volume: int(d.Volume),
		})
	}
	return out, nil
}

// WaitForDevice polls until a device with the given name appears, returning its
// ID. Used to discover the librespot "AudioPulse" device after it connects.
func (c *Client) WaitForDevice(ctx context.Context, name string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		devices, err := c.Devices(ctx)
		if err == nil {
			for _, d := range devices {
				if d.Name == name {
					return d.ID, nil
				}
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return "", fmt.Errorf("waiting for device %q: %w", name, err)
			}
			return "", fmt.Errorf("device %q did not appear within %s", name, timeout)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Transfer moves playback to a device (and optionally starts playing).
func (c *Client) Transfer(ctx context.Context, deviceID string, play bool) error {
	return c.api.TransferPlayback(ctx, zspotify.ID(deviceID), play)
}

// PlayContext starts a playlist/album context on a device, optionally at a
// specific track URI offset.
func (c *Client) PlayContext(ctx context.Context, deviceID string, contextURI zspotify.URI, offset zspotify.URI) error {
	did := zspotify.ID(deviceID)
	opt := &zspotify.PlayOptions{DeviceID: &did, PlaybackContext: &contextURI}
	if offset != "" {
		opt.PlaybackOffset = &zspotify.PlaybackOffset{URI: offset}
	}
	return c.api.PlayOpt(ctx, opt)
}

// PlayTracksAt plays an explicit list of track URIs on a device, starting at
// the track at index pos so playback continues through the list.
func (c *Client) PlayTracksAt(ctx context.Context, deviceID string, uris []zspotify.URI, pos int) error {
	did := zspotify.ID(deviceID)
	opt := &zspotify.PlayOptions{DeviceID: &did, URIs: uris}
	if pos > 0 {
		opt.PlaybackOffset = &zspotify.PlaybackOffset{Position: &pos}
	}
	return c.api.PlayOpt(ctx, opt)
}

// Resume resumes playback (on a device if given).
func (c *Client) Resume(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return c.api.Play(ctx)
	}
	did := zspotify.ID(deviceID)
	return c.api.PlayOpt(ctx, &zspotify.PlayOptions{DeviceID: &did})
}

// Pause pauses playback.
func (c *Client) Pause(ctx context.Context) error { return c.api.Pause(ctx) }

// Next skips to the next track.
func (c *Client) Next(ctx context.Context) error { return c.api.Next(ctx) }

// Previous skips to the previous track.
func (c *Client) Previous(ctx context.Context) error { return c.api.Previous(ctx) }

// Seek seeks to a position within the current track.
func (c *Client) Seek(ctx context.Context, pos time.Duration) error {
	return c.api.Seek(ctx, int(pos/time.Millisecond))
}

// SetVolume sets the playback volume (0-100).
func (c *Client) SetVolume(ctx context.Context, percent int) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return c.api.Volume(ctx, percent)
}

// SetShuffle toggles shuffle.
func (c *Client) SetShuffle(ctx context.Context, on bool) error {
	return c.api.Shuffle(ctx, on)
}

// SetRepeat sets the repeat mode: "off", "context" (loop all), or "track"
// (loop one).
func (c *Client) SetRepeat(ctx context.Context, state string) error {
	return c.api.Repeat(ctx, state)
}

// State returns the current playback snapshot, or (nil, nil) when nothing is
// active.
func (c *Client) State(ctx context.Context) (*PlayerState, error) {
	ps, err := c.api.PlayerState(ctx)
	if err != nil {
		return nil, err
	}
	if ps == nil {
		return nil, nil
	}
	st := &PlayerState{
		Playing:    ps.Playing,
		Progress:   time.Duration(ps.Progress) * time.Millisecond,
		Volume:     int(ps.Device.Volume),
		Shuffle:    ps.ShuffleState,
		Repeat:     ps.RepeatState,
		DeviceID:   string(ps.Device.ID),
		DeviceName: ps.Device.Name,
	}
	if ps.Item != nil {
		t := toTrack(ps.Item)
		st.Track = &t
	}
	return st, nil
}

// --- conversions -------------------------------------------------------------

func toTrack(t *zspotify.FullTrack) Track {
	tr := Track{
		ID:       t.ID,
		URI:      t.URI,
		Title:    t.Name,
		Album:    t.Album.Name,
		Duration: time.Duration(t.Duration) * time.Millisecond,
		Artist:   joinArtists(t.Artists),
	}
	if len(t.Album.Images) > 0 {
		tr.ImageURL = t.Album.Images[0].URL
	}
	return tr
}

func simpleToTrack(t *zspotify.SimpleTrack) Track {
	return Track{
		ID:       t.ID,
		URI:      t.URI,
		Title:    t.Name,
		Duration: time.Duration(t.Duration) * time.Millisecond,
		Artist:   joinArtists(t.Artists),
	}
}

func joinArtists(artists []zspotify.SimpleArtist) string {
	names := make([]string, 0, len(artists))
	for _, a := range artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}
