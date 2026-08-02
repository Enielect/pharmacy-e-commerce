package handlers

import (
	"net/http"
)

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	pd.Title = "Login"
	h.render(w, r, "login", pd)
}

func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := h.authStore.Authenticate(email, password)
	if err != nil {
		pd := h.pageData(r)
		pd.Title = "Login"
		pd.Error = "Invalid email or password"
		h.render(w, r, "login", pd)
		return
	}

	token, err := h.authStore.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   86400 * 7,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	pd.Title = "Register"
	h.render(w, r, "register", pd)
}

func (h *Handler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	_, err := h.authStore.CreateUser(email, password, "customer")
	if err != nil {
		pd := h.pageData(r)
		pd.Title = "Register"
		pd.Error = "Registration failed: " + err.Error()
		h.render(w, r, "register", pd)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	token := h.sessionToken(r)
	if token != "" {
		h.authStore.DeleteSession(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
