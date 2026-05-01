package api

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	dflhttp "github.com/duffleone/dfl/http"
)

// User is the resource shape on the wire.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Users shows handlers with path params, query params, JSON body, and
// path+body together.
type Users struct {
	mu    sync.Mutex
	store map[string]User
	next  int
}

// NewUsers returns a Users with an empty in-memory store.
func NewUsers() *Users {
	return &Users{store: map[string]User{}}
}

// Mount wires up user endpoints on rg.
func (u *Users) Mount(rg *dflhttp.Router) {
	dflhttp.Handle(rg, http.MethodGet, "/users", u.handleList)
	dflhttp.Handle(rg, http.MethodGet, "/users/{id}", u.handleGet)
	dflhttp.Handle(rg, http.MethodPost, "/users", u.handleCreate)
	dflhttp.Handle(rg, http.MethodPut, "/users/{id}", u.handleUpdate)
}

// ListUsersReq pulls all its fields from the query string.
type ListUsersReq struct {
	Limit  int    `query:"limit"`
	Cursor string `query:"cursor"`
}

// ListUsersResp wraps the slice so the handler resp can be a pointer to a
// struct, in line with the rest of the API.
type ListUsersResp struct {
	Users []User `json:"users"`
}

func (u *Users) handleList(_ context.Context, req *ListUsersReq) (*ListUsersResp, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	out := make([]User, 0, limit)

	for _, user := range u.store {
		if len(out) >= limit {
			break
		}

		out = append(out, user)
	}

	return &ListUsersResp{Users: out}, nil
}

// GetUserReq pulls its only field from the URL path.
type GetUserReq struct {
	ID string `path:"id"`
}

func (u *Users) handleGet(_ context.Context, req *GetUserReq) (*User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	user, ok := u.store[req.ID]
	if !ok {
		return nil, dflhttp.New(http.StatusNotFound, "user_not_found", dflhttp.M{"id": req.ID})
	}

	return &user, nil
}

// CreateUserReq pulls its only field from the JSON body.
type CreateUserReq struct {
	Name string `json:"name"`
}

func (u *Users) handleCreate(_ context.Context, req *CreateUserReq) (*User, error) {
	if req.Name == "" {
		return nil, dflhttp.New(http.StatusBadRequest, "name_required", nil)
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	u.next++
	user := User{
		ID:   strconv.Itoa(u.next),
		Name: req.Name,
	}
	u.store[user.ID] = user

	return &user, nil
}

// UpdateUserReq mixes a path param and a JSON body field.
type UpdateUserReq struct {
	ID   string `path:"id"`
	Name string `json:"name"`
}

func (u *Users) handleUpdate(_ context.Context, req *UpdateUserReq) (*User, error) {
	if req.Name == "" {
		return nil, dflhttp.New(http.StatusBadRequest, "name_required", nil)
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	user, ok := u.store[req.ID]
	if !ok {
		return nil, dflhttp.New(http.StatusNotFound, "user_not_found", dflhttp.M{"id": req.ID})
	}

	user.Name = req.Name
	u.store[user.ID] = user

	return &user, nil
}
