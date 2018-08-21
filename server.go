package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/zmb3/spotify"
	"encoding/json"
	"sort"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:3000/callback"

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadPrivate)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

var clientReal *spotify.Client = nil

type PlaylistTracks []spotify.PlaylistTrack

func (slice PlaylistTracks) Len() int {
	return len(slice)
}

func (slice PlaylistTracks) Less(i, j int) bool {
	return slice[i].Track.Popularity > slice[j].Track.Popularity
}

func (slice PlaylistTracks) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func GetClientPlaylists(client *spotify.Client) (string, error){
	user, err := client.CurrentUser()
	if (err != nil){
		return "", err
	}

	userURI := user.ID

	playlists, err := client.CurrentUsersPlaylists()
	if (err != nil){
		return "",err
	}

	var maxPlaylistURI spotify.ID
	max := 0
	for _, playlist := range playlists.Playlists{
		num := int(playlist.Tracks.Total)
		if (num > max){
			max = num
			maxPlaylistURI = playlist.ID
		}
	}

	playListTracks := make(PlaylistTracks, 0)
	fields := "items( is_local, track(artists(name),name,album(images(url)),popularity))"

	for index := 0 ;index <= max; index += 100 {
		options := spotify.Options{nil,nil,&index}
		fullPlaylist, err := client.GetPlaylistTracksOpt(userURI, maxPlaylistURI, &options, fields)
		if (err != nil){
			return "", err
		}

		for _,track := range fullPlaylist.Tracks {
			playListTracks = append(playListTracks, track)
		}
	}

	sort.Sort(playListTracks)

	jsonForm, jsonErr := json.Marshal(playListTracks)
	if (jsonErr != nil){
		return "", jsonErr
	}

	return string(jsonForm), nil
}

func main() {
	auth.SetAuthInfo(os.Getenv("API_KEY"), os.Getenv("API_SECRET"))

	go http.ListenAndServe(":3000", nil)

	http.HandleFunc("/callback", completeAuth)

	http.Handle("/", http.FileServer(http.Dir("./web")))

	http.HandleFunc("/mosaic", func(w http.ResponseWriter, r *http.Request){
		if (clientReal != nil){
			playlists, playlistsErr := GetClientPlaylists(clientReal)
			if (playlistsErr != nil){
				fmt.Fprintf(w, playlistsErr.Error())
				return
			}
			fmt.Fprintf(w, playlists)
		} else {
			url := auth.AuthURL(state)
			fmt.Fprintf(w, "%s", url)
		}
	})

	msg := <- ch
	clientReal = msg

	var input string
	fmt.Scanln(&input)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	log.Println("Login completed")
	ch <- &client

	http.ServeFile(w,r,"./web/index.html")
}
