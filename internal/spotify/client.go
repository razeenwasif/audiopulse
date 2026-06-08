// Package spotify wraps the Spotify Web API (via zmb3/spotify) behind small
// domain types, so the UI never depends on the library directly.
//
// The Web API is used purely for control and metadata — actual audio is played
// by the librespot device this client targets.
package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	zspotify "github.com/zmb3/spotify/v2"
)

// maxLibraryItems caps how many items a paginated library fetch will collect, a
// safety bound against pathologically large libraries (the Spotify API pages in
// 50–100s, so this is many round-trips before it ever trips).
const maxLibraryItems = 10000

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

// Show is a saved podcast.
type Show struct {
	ID        zspotify.ID
	URI       zspotify.URI
	Name      string
	Publisher string
	ImageURL  string
}

// Episode is a single podcast episode.
type Episode struct {
	ID       zspotify.ID
	URI      zspotify.URI
	Title    string
	ShowName string
	Date     string // release date, e.g. "2026-06-01"
	Duration time.Duration
	ImageURL string
	Playable bool // false → region-locked / externally hosted; may not play
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

// Playlists returns the user's playlists, following pagination so libraries
// larger than one API page are returned in full.
func (c *Client) Playlists(ctx context.Context) ([]Playlist, error) {
	page, err := c.api.CurrentUsersPlaylists(ctx, zspotify.Limit(50))
	if err != nil {
		return nil, err
	}
	var out []Playlist
	for {
		for _, p := range page.Playlists {
			out = append(out, Playlist{
				ID:    p.ID,
				URI:   p.URI,
				Name:  p.Name,
				Count: int(p.Tracks.Total),
			})
		}
		if len(out) >= maxLibraryItems {
			break
		}
		if err := c.api.NextPage(ctx, page); err != nil {
			if errors.Is(err, zspotify.ErrNoMorePages) {
				break
			}
			return out, err
		}
	}
	return out, nil
}

// Page sizes for streamed track listings: the API maxima for each endpoint.
const (
	likedPageSize    = 50
	playlistPageSize = 100
)

// TrackPage is one page of a paginated track listing. The caller streams pages
// by re-requesting at NextOffset while HasMore reports true.
type TrackPage struct {
	Tracks []Track // playable tracks in this page (unplayable items are skipped)
	Offset int     // raw item offset this page began at
	Total  int     // total items available across all pages

	next int  // raw offset for the following page
	more bool // whether any items remain after this page
}

// HasMore reports whether further pages remain.
func (p TrackPage) HasMore() bool { return p.more }

// NextOffset is the offset to request for the following page.
func (p TrackPage) NextOffset() int { return p.next }

// LikedSongsPage returns one page of the user's saved tracks starting at offset.
// Stream the full collection by following HasMore/NextOffset.
func (c *Client) LikedSongsPage(ctx context.Context, offset int) (TrackPage, error) {
	page, err := c.api.CurrentUsersTracks(ctx, zspotify.Limit(likedPageSize), zspotify.Offset(offset))
	if err != nil {
		return TrackPage{}, err
	}
	out := make([]Track, 0, len(page.Tracks))
	for i := range page.Tracks {
		out = append(out, toTrack(&page.Tracks[i].FullTrack))
	}
	return newTrackPage(out, offset, len(page.Tracks), int(page.Total)), nil
}

// PlaylistTracksPage returns one page of a playlist's tracks starting at offset.
// Stream the full playlist by following HasMore/NextOffset.
func (c *Client) PlaylistTracksPage(ctx context.Context, id zspotify.ID, offset int) (TrackPage, error) {
	page, err := c.api.GetPlaylistItems(ctx, id, zspotify.Limit(playlistPageSize), zspotify.Offset(offset))
	if err != nil {
		return TrackPage{}, err
	}
	out := make([]Track, 0, len(page.Items))
	for _, it := range page.Items {
		if it.Track.Track != nil {
			out = append(out, toTrack(it.Track.Track))
		}
	}
	// Advance by the raw item count, not len(out): unplayable items are filtered
	// from Tracks but still consume an offset slot.
	return newTrackPage(out, offset, len(page.Items), int(page.Total)), nil
}

// newTrackPage computes pagination state from the raw item count this page
// consumed (raw), independent of how many were kept as playable tracks.
func newTrackPage(tracks []Track, offset, raw, total int) TrackPage {
	next := offset + raw
	return TrackPage{
		Tracks: tracks,
		Offset: offset,
		Total:  total,
		next:   next,
		more:   raw > 0 && next < total,
	}
}

// maxShowEpisodes bounds how many episodes are fetched per show (most-recent
// first); shows can have thousands and only the recent ones matter here.
const maxShowEpisodes = 200

// SavedShows returns the user's saved podcasts, following pagination.
func (c *Client) SavedShows(ctx context.Context) ([]Show, error) {
	page, err := c.api.CurrentUsersShows(ctx, zspotify.Limit(50))
	if err != nil {
		return nil, err
	}
	var out []Show
	for {
		for i := range page.Shows {
			s := &page.Shows[i]
			out = append(out, Show{
				ID:        s.ID,
				URI:       s.URI,
				Name:      s.Name,
				Publisher: s.Publisher,
				ImageURL:  firstImageURL(s.Images),
			})
		}
		if len(out) >= maxLibraryItems {
			break
		}
		if err := c.api.NextPage(ctx, page); err != nil {
			if errors.Is(err, zspotify.ErrNoMorePages) {
				break
			}
			return out, err
		}
	}
	return out, nil
}

// ShowEpisodes returns a show's episodes (most-recent first), up to a cap.
func (c *Client) ShowEpisodes(ctx context.Context, id zspotify.ID) ([]Episode, error) {
	page, err := c.api.GetShowEpisodes(ctx, string(id), zspotify.Limit(50))
	if err != nil {
		return nil, err
	}
	var out []Episode
	for {
		for i := range page.Episodes {
			e := &page.Episodes[i]
			out = append(out, Episode{
				ID:       e.ID,
				URI:      e.URI,
				Title:    e.Name,
				ShowName: e.Show.Name,
				Date:     e.ReleaseDate,
				Duration: time.Duration(e.Duration_ms) * time.Millisecond,
				ImageURL: firstImageURL(e.Images),
				Playable: e.IsPlayable,
			})
		}
		if len(out) >= maxShowEpisodes {
			break
		}
		if err := c.api.NextPage(ctx, page); err != nil {
			if errors.Is(err, zspotify.ErrNoMorePages) {
				break
			}
			return out, err
		}
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

// FindDevice returns the ID of a currently-available device with the given name,
// or "" if none is present right now (single-shot, no waiting). Used to
// re-resolve the librespot device after a restart or drop.
func (c *Client) FindDevice(ctx context.Context, name string) (string, error) {
	devices, err := c.Devices(ctx)
	if err != nil {
		return "", err
	}
	for _, d := range devices {
		if d.Name == name {
			return d.ID, nil
		}
	}
	return "", nil
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

// AddToQueue queues a track to play after the current one, on the given device.
func (c *Client) AddToQueue(ctx context.Context, trackID zspotify.ID, deviceID string) error {
	var opt *zspotify.PlayOptions
	if deviceID != "" {
		did := zspotify.ID(deviceID)
		opt = &zspotify.PlayOptions{DeviceID: &did}
	}
	return c.api.QueueSongOpt(ctx, trackID, opt)
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

func firstImageURL(images []zspotify.Image) string {
	if len(images) > 0 {
		return images[0].URL
	}
	return ""
}

func joinArtists(artists []zspotify.SimpleArtist) string {
	names := make([]string, 0, len(artists))
	for _, a := range artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}
