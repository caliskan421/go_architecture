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

func TestAuthor_RequiresAuth(t *testing.T) {
	ta := testutil.NewTestApp(t)
	resp := ta.Get(t, "/api/authors", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthor_UserCannotWrite_OnlyRead(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginUser(t)

	// user role'ünde sadece "read" izni var → yazma denemesi 403 dönmeli
	resp := ta.Post(t, "/api/authors", map[string]string{
		"name": "X", "description": "y",
	}, cookie)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// read uçu çalışmalı
	resp = ta.Get(t, "/api/authors", cookie)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthor_AdminCRUD_HappyPath(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	// Create
	createBody := map[string]string{"name": "Yaşar Kemal", "description": "İnce Memed yazarı"}
	resp := ta.Post(t, "/api/authors", createBody, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created dto.AuthorResponse
	testutil.DecodeData(t, resp, &created)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "Yaşar Kemal", created.Name)

	// Get
	resp = ta.Get(t, "/api/authors/"+itoa(created.ID), cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got dto.AuthorResponse
	testutil.DecodeData(t, resp, &got)
	assert.Equal(t, created.ID, got.ID)

	// Update
	resp = ta.Put(t, "/api/authors/"+itoa(created.ID), map[string]string{
		"name": "Yaşar K.", "description": "kısaldı",
	}, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var updated dto.AuthorResponse
	testutil.DecodeData(t, resp, &updated)
	assert.Equal(t, "Yaşar K.", updated.Name)

	// List (paginate yapısını doğrula)
	resp = ta.Get(t, "/api/authors", cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page httpx.Page[dto.AuthorResponse]
	testutil.DecodeData(t, resp, &page)
	assert.Equal(t, int64(1), page.Total)
	assert.Len(t, page.Items, 1)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 20, page.PageSize)

	// Delete
	resp = ta.Delete(t, "/api/authors/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Get sonra → 404
	resp = ta.Get(t, "/api/authors/"+itoa(created.ID), cookie)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAuthor_DuplicateName_Returns409(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	body := map[string]string{"name": "Aynı İsim", "description": ""}
	resp := ta.Post(t, "/api/authors", body, cookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = ta.Post(t, "/api/authors", body, cookie)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "conflict", code)
}

func TestAuthor_List_WithSearchAndSort(t *testing.T) {
	ta := testutil.NewTestApp(t)
	cookie := ta.LoginAdmin(t)

	// 3 author yarat
	for _, name := range []string{"Ahmet", "Burak", "Cem"} {
		resp := ta.Post(t, "/api/authors", map[string]string{"name": name}, cookie)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// q=hmet → sadece "Ahmet"
	resp := ta.Get(t, "/api/authors?q=hmet", cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page httpx.Page[dto.AuthorResponse]
	testutil.DecodeData(t, resp, &page)
	require.Equal(t, int64(1), page.Total)
	assert.Equal(t, "Ahmet", page.Items[0].Name)

	// sort=name&order=desc → Cem, Burak, Ahmet
	resp = ta.Get(t, "/api/authors?sort=name&order=desc", cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &page)
	require.Len(t, page.Items, 3)
	assert.Equal(t, "Cem", page.Items[0].Name)
	assert.Equal(t, "Ahmet", page.Items[2].Name)

	// page_size=2&page=2 → son satır
	resp = ta.Get(t, "/api/authors?sort=name&order=asc&page=2&page_size=2", cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.DecodeData(t, resp, &page)
	assert.Equal(t, int64(3), page.Total)
	assert.Equal(t, 2, page.Page)
	assert.Len(t, page.Items, 1)
	assert.Equal(t, "Cem", page.Items[0].Name)
}

// itoa, küçük yardımcı: id'yi path'e koymak için.
// fmt.Sprintf("%d", n) yerine açık olsun diye.
func itoa(n uint) string {
	return uintToStr(n)
}

func uintToStr(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
