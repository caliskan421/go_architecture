package handler_test

import (
	"net/http"
	"testing"

	"libra_management/internal/dto"
	"libra_management/internal/httpx"
	"libra_management/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createAuthor, helper: test fixture olarak bir author yaratır.
func createAuthor(t *testing.T, ta *testutil.TestApp, cookie, name string) dto.AuthorResponse {
	t.Helper()
	resp := ta.Post(t, "/api/authors", map[string]string{"name": name}, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var a dto.AuthorResponse
	testutil.DecodeData(t, resp, &a)
	return a
}

func TestBook_AdminCRUD_HappyPath(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	author := createAuthor(t, ta, cookie, "Sabahattin Ali")

	// Create
	createBody := map[string]interface{}{
		"author_id":   author.ID,
		"name":        "Kürk Mantolu Madonna",
		"description": "Berlin'de geçen bir aşk hikâyesi",
	}
	resp := ta.Post(t, "/api/books", createBody, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created dto.BookResponse
	testutil.DecodeData(t, resp, &created)
	assert.Equal(t, "Kürk Mantolu Madonna", created.Name)
	assert.Equal(t, author.ID, created.AuthorID)
	assert.Equal(t, "Sabahattin Ali", created.Author.Name, "preload Author dolmalı")

	// List
	resp = ta.Get(t, "/api/books", cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page httpx.Page[dto.BookResponse]
	testutil.DecodeData(t, resp, &page)
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "Sabahattin Ali", page.Items[0].Author.Name)

	// Update — kitabı farklı author'a taşı
	other := createAuthor(t, ta, cookie, "Yusuf Atılgan")
	resp = ta.Put(t, "/api/books/"+itoa(created.ID), map[string]interface{}{
		"author_id": other.ID, "name": "Aylak Adam", "description": "C.",
	}, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var updated dto.BookResponse
	testutil.DecodeData(t, resp, &updated)
	assert.Equal(t, other.ID, updated.AuthorID)

	// Delete
	resp = ta.Delete(t, "/api/books/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestBook_NonexistentAuthor_Returns400Validation(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	resp := ta.Post(t, "/api/books", map[string]interface{}{
		"author_id":   9999,
		"name":        "Hayalet Yazarın Kitabı",
		"description": "",
	}, cookie)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "validation_error", code)
}

func TestBook_FilterByAuthorID(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	a1 := createAuthor(t, ta, cookie, "A1")
	a2 := createAuthor(t, ta, cookie, "A2")

	for _, n := range []string{"K1", "K2"} {
		resp := ta.Post(t, "/api/books", map[string]interface{}{
			"author_id": a1.ID, "name": n + "-a1",
		}, cookie)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}
	resp := ta.Post(t, "/api/books", map[string]interface{}{
		"author_id": a2.ID, "name": "tek-a2",
	}, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// a1'in kitapları
	resp = ta.Get(t, "/api/books?author_id="+itoa(a1.ID), cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page httpx.Page[dto.BookResponse]
	testutil.DecodeData(t, resp, &page)
	assert.Equal(t, int64(2), page.Total)

	// a2'nin kitapları
	resp = ta.Get(t, "/api/books?author_id="+itoa(a2.ID), cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &page)
	assert.Equal(t, int64(1), page.Total)
}
