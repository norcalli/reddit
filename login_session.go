// Copyright 2012 Jimmy Zelinskie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reddit implements an abstraction for the reddit.com API.
package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// LoginSession represents an HTTP session with reddit.com --
// all authenticated API calls are methods bound to this type.
type LoginSession struct {
	username  string
	password  string
	useragent string
	cookie    *http.Cookie
	modhash   string `json:"modhash"`
	Session
}

// NewLoginSession creates a new session for those who want to log into a
// reddit account.
func NewLoginSession(username, password, useragent string) (*LoginSession, error) {
	session := &LoginSession{
		username:  username,
		password:  password,
		useragent: useragent,
		Session:   Session{useragent},
	}

	loginURL := fmt.Sprintf("http://www.reddit.com/api/login/%s", username)
	postValues := url.Values{
		"user":     {username},
		"passwd":   {password},
		"api_type": {"json"},
	}
	resp, err := http.PostForm(loginURL, postValues)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	// Get the session cookie.
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "reddit_session" {
			session.cookie = cookie
		}
	}

	// Get the modhash from the JSON.
	type Response struct {
		JSON struct {
			Errors [][]string
			Data   struct {
				Modhash string
			}
		}
	}

	r := &Response{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	if len(r.JSON.Errors) != 0 {
		var msg []string
		for _, k := range r.JSON.Errors {
			msg = append(msg, k[1])
		}
		return nil, errors.New(strings.Join(msg, ", "))
	}
	session.modhash = r.JSON.Data.Modhash

	return session, nil
}

// Clear clears all session cookies and updates the current session with a new one.
func (s LoginSession) Clear() error {
	req := &request{
		url: "http://www.reddit.com/api/clear_sessions",
		values: &url.Values{
			"curpass": {s.password},
			"uh":      {s.modhash},
		},
		useragent: s.useragent,
	}
	body, err := req.getResponse()
	if err != nil {
		return err
	}

	if !strings.Contains(body.String(), "all other sessions have been logged out") {
		return errors.New("failed to clear session")
	}
	return nil
}

// Frontpage returns the submissions on the logged-in user's personal frontpage.
func (s LoginSession) Frontpage() ([]*Submission, error) {
	req := request{
		url:       "http://www.reddit.com/.json",
		cookie:    s.cookie,
		useragent: s.useragent,
	}
	body, err := req.getResponse()
	if err != nil {
		return nil, err
	}

	type Response struct {
		Data struct {
			Children []struct {
				Data *Submission
			}
		}
	}
	r := &Response{}
	err = json.NewDecoder(body).Decode(r)
	if err != nil {
		return nil, err
	}

	submissions := make([]*Submission, len(r.Data.Children))
	for i, child := range r.Data.Children {
		submissions[i] = child.Data
	}

	return submissions, nil
}

// Me returns an up-to-date redditor object of the logged-in user.
func (s LoginSession) Me() (*Redditor, error) {
	req := &request{
		url:       "http://www.reddit.com/api/me.json",
		cookie:    s.cookie,
		useragent: s.useragent,
	}
	body, err := req.getResponse()
	if err != nil {
		return nil, err
	}

	type Response struct {
		Data Redditor
	}
	r := &Response{}
	err = json.NewDecoder(body).Decode(r)
	if err != nil {
		return nil, err
	}

	return &r.Data, nil
}

// Vote either votes or rescinds a vote for a Submission or Comment.
func (s LoginSession) Vote(v Voter, vote vote) error {
	req := &request{
		url: "http://www.reddit.com/api/vote",
		values: &url.Values{
			"id":  {v.voteID()},
			"dir": {string(vote)},
			"uh":  {s.modhash},
		},
		cookie:    s.cookie,
		useragent: s.useragent,
	}
	body, err := req.getResponse()
	if err != nil {
		return err
	}
	if body.String() != "{}" {
		return errors.New("failed to vote")
	}
	return nil
}

// Reply posts a comment as a response to a Submission or Comment.
func (s LoginSession) Reply(r Replier, comment string) error {
	req := &request{
		url: "http://www.reddit.com/api/comment",
		values: &url.Values{
			"thing_id": {r.replyID()},
			"text":     {comment},
			"uh":       {s.modhash},
		},
		cookie:    s.cookie,
		useragent: s.useragent,
	}

	body, err := req.getResponse()
	if err != nil {
		return err
	}

	if !strings.Contains(body.String(), "data") {
		return errors.New("failed to post comment")
	}

	return nil
}

// Delete deletes a Submission or Comment.
func (s LoginSession) Delete(d Deleter) error {
	req := &request{
		url: "http://www.reddit.com/api/del",
		values: &url.Values{
			"id": {d.deleteID()},
			"uh": {s.modhash},
		},
		cookie:    s.cookie,
		useragent: s.useragent,
	}

	body, err := req.getResponse()
	if err != nil {
		return err
	}

	if !strings.Contains(body.String(), "data") {
		return errors.New("failed to delete item")
	}

	return nil
}
