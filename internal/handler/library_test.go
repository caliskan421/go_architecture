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

func createBook(t *testing.T, ta *testutil.TestApp, cookie string, authorID uint, name string) dto.BookResponse {
	t.Helper()
	resp := ta.Post(t, "/api/books", map[string]interface{}{
		"author_id": authorID, "name": name,
	}, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var b dto.BookResponse
	testutil.DecodeData(t, resp, &b)
	return b
}

func TestLibrary_CreateWithBooks_HappyPath(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)
	a := createAuthor(t, ta, cookie, "Author One")
	b1 := createBook(t, ta, cookie, a.ID, "B1")
	b2 := createBook(t, ta, cookie, a.ID, "B2")

	resp := ta.Post(t, "/api/libraries", map[string]interface{}{
		"name":     "Merkez Kütüphane",
		"book_ids": []uint{b1.ID, b2.ID},
	}, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var lib dto.LibraryResponse
	testutil.DecodeData(t, resp, &lib)
	assert.Equal(t, "Merkez Kütüphane", lib.Name)
	assert.Len(t, lib.Books, 2)
}

func TestLibrary_CreateWithMissingBookID_Returns400(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	resp := ta.Post(t, "/api/libraries", map[string]interface{}{
		"name":     "Boş",
		"book_ids": []uint{9999},
	}, cookie)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "validation_error", code)
}

func TestLibrary_AddRemoveBooks(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)
	a := createAuthor(t, ta, cookie, "Author One")
	b1 := createBook(t, ta, cookie, a.ID, "B1")
	b2 := createBook(t, ta, cookie, a.ID, "B2")
	b3 := createBook(t, ta, cookie, a.ID, "B3")

	// Library boş yarat
	resp := ta.Post(t, "/api/libraries", map[string]interface{}{
		"name": "İlçe Kütüphanesi",
	}, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var lib dto.LibraryResponse
	testutil.DecodeData(t, resp, &lib)
	assert.Empty(t, lib.Books)

	// Add b1, b2
	resp = ta.Post(t, "/api/libraries/"+itoa(lib.ID)+"/books",
		map[string]interface{}{"book_ids": []uint{b1.ID, b2.ID}}, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &lib)
	assert.Len(t, lib.Books, 2)

	// Add b2 again + b3 → b2 sessizce yok sayılır, b3 eklenir
	resp = ta.Post(t, "/api/libraries/"+itoa(lib.ID)+"/books",
		map[string]interface{}{"book_ids": []uint{b2.ID, b3.ID}}, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &lib)
	assert.Len(t, lib.Books, 3)

	// Remove b1
	resp = ta.Delete(t, "/api/libraries/"+itoa(lib.ID)+"/books",
		map[string]interface{}{"book_ids": []uint{b1.ID}}, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &lib)
	assert.Len(t, lib.Books, 2)
}

func TestLibrary_Update_ReplacesBookSet(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)
	a := createAuthor(t, ta, cookie, "Author One")
	b1 := createBook(t, ta, cookie, a.ID, "B1")
	b2 := createBook(t, ta, cookie, a.ID, "B2")

	resp := ta.Post(t, "/api/libraries", map[string]interface{}{
		"name":     "L1",
		"book_ids": []uint{b1.ID, b2.ID},
	}, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var lib dto.LibraryResponse
	testutil.DecodeData(t, resp, &lib)

	// Update: book_ids:[b1] → b2 ilişkisi koparılmalı
	resp = ta.Put(t, "/api/libraries/"+itoa(lib.ID), map[string]interface{}{
		"name":     "L1-renamed",
		"book_ids": []uint{b1.ID},
	}, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &lib)
	assert.Equal(t, "L1-renamed", lib.Name)
	assert.Len(t, lib.Books, 1)
	assert.Equal(t, b1.ID, lib.Books[0].ID)
}

func TestLibrary_List_Paginated(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	for _, n := range []string{"L1", "L2", "L3"} {
		resp := ta.Post(t, "/api/libraries", map[string]interface{}{"name": n}, cookie)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	resp := ta.Get(t, "/api/libraries?page_size=2", cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page httpx.Page[dto.LibraryResponse]
	testutil.DecodeData(t, resp, &page)
	assert.Equal(t, int64(3), page.Total)
	assert.Equal(t, 2, page.PageSize)
	assert.Equal(t, 2, page.TotalPages)
	assert.Len(t, page.Items, 2)
}
