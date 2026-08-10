package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"pharmacy/internal/auth"
	"pharmacy/internal/cart"
	"pharmacy/internal/catalog"
	"pharmacy/internal/config"
	"pharmacy/internal/imagestore"
	"pharmacy/internal/orders"
	"pharmacy/internal/payments"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	cfg          *config.Config
	log          *slog.Logger
	authStore    *auth.Store
	catalogStore *catalog.Store
	cartStore    *cart.Store
	orderStore   *orders.Store
	paymentProv  payments.PaymentProvider
	imageStore   imagestore.ImageStore
	templates    map[string]*template.Template
}

func New(
	cfg *config.Config,
	log *slog.Logger,
	authStore *auth.Store,
	catalogStore *catalog.Store,
	cartStore *cart.Store,
	orderStore *orders.Store,
	paymentProv payments.PaymentProvider,
	imageStore imagestore.ImageStore,
	templatesFS embed.FS,
) *Handler {
	h := &Handler{
		cfg:          cfg,
		log:          log,
		authStore:    authStore,
		catalogStore: catalogStore,
		cartStore:    cartStore,
		orderStore:   orderStore,
		paymentProv:  paymentProv,
		imageStore:   imageStore,
		templates:    make(map[string]*template.Template),
	}
	h.loadTemplates(templatesFS)
	return h
}

func (h *Handler) loadTemplates(templatesFS embed.FS) {
	layouts := []string{"templates/layout.html"}

	funcMap := template.FuncMap{
		"centsToDollars": func(c int32) string {
			d := float64(c) / 100.0
			return fmt.Sprintf("%.2f", d)
		},
		"imageURL": func(key string) string {
			if key == "" {
				return "/static/placeholder.svg"
			}
			if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
				return key
			}
			return h.imageStore.URL(key)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int32) int32 { return a * b },
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"seq": func(start, end int) []int {
			var s []int
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	base := template.New("").Funcs(funcMap)
	base = template.Must(base.ParseFS(templatesFS, layouts...))

	pages := map[string][]string{
		"home":            {"templates/pages/home.html", "templates/partials/product_grid.html", "templates/partials/product_card.html", "templates/partials/pagination.html"},
		"product":         {"templates/pages/product.html"},
		"cart":            {"templates/pages/cart.html", "templates/partials/cart_drawer.html"},
		"checkout":        {"templates/pages/checkout.html"},
		"login":           {"templates/pages/login.html"},
		"register":        {"templates/pages/register.html"},
		"orders":          {"templates/pages/orders.html"},
		"order":           {"templates/pages/order.html"},
		"admin_drafts":    {"templates/pages/admin/drafts.html"},
		"admin_draft":     {"templates/pages/admin/draft.html"},
		"admin_inventory": {"templates/pages/admin/inventory.html"},
		"admin_orders":    {"templates/pages/admin/orders.html"},
		"product_grid":    {"templates/partials/product_grid.html", "templates/partials/product_card.html", "templates/partials/pagination.html"},
		"cart_drawer":     {"templates/partials/cart_drawer.html"},
		"cart_badge":      {"templates/partials/cart_badge.html"},
	}

	for name, files := range pages {
		allFiles := append(layouts, files...)
		tmpl := template.Must(template.Must(base.Clone()).ParseFS(templatesFS, allFiles...))
		h.templates[name] = tmpl
	}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.log.Error("render error", "template", name, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, filepath.Base(name)+".html", data)
}

func (h *Handler) sessionToken(r *http.Request) string {
	if c, err := r.Cookie("session"); err == nil {
		return c.Value
	}
	return ""
}

func (h *Handler) currentUser(r *http.Request) *auth.User {
	return auth.UserFromContext(r.Context())
}

func (h *Handler) userID(u *auth.User) *int64 {
	if u != nil {
		return &u.ID
	}
	return nil
}

func (h *Handler) SetupRoutes(r chi.Router) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(h.authStore.OptionalAuth)

	fileServer := http.FileServer(http.Dir(h.cfg.UploadDir))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", fileServer))
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Get("/", h.HomePage)
	r.Get("/products/{slug}", h.ProductDetail)
	r.Get("/search", h.SearchResults)

	r.Get("/login", h.LoginPage)
	r.Post("/login", h.LoginPost)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.RegisterPost)
	r.Post("/logout", h.LogoutPost)

	r.Get("/cart", h.CartPage)
	r.Post("/cart/add", h.CartAdd)
	r.Post("/cart/update/{itemID}", h.CartUpdate)
	r.Post("/cart/remove/{itemID}", h.CartRemove)
	r.Get("/cart/count", h.CartCount)
	r.Get("/cart/drawer", h.CartDrawer)

	r.Group(func(r chi.Router) {
		r.Use(h.authStore.RequireAuth)
		r.Get("/checkout", h.CheckoutPage)
		r.Post("/checkout", h.CheckoutPost)
		r.Get("/checkout/pay/{orderID}", h.PaymentPage)
		r.Post("/checkout/pay/{orderID}", h.PaymentPost)
		r.Get("/orders", h.OrderHistory)
		r.Get("/orders/{orderID}", h.OrderDetail)
	})

	r.Group(func(r chi.Router) {
		r.Use(h.authStore.RequireAuth)
		r.Use(h.authStore.RequireRole("admin", "pharmacist"))
		r.Get("/admin", h.AdminDashboard)
		r.Get("/admin/drafts", h.AdminDrafts)
		r.Get("/admin/drafts/{id}", h.AdminDraftDetail)
		r.Post("/admin/drafts/{id}/approve", h.AdminDraftApprove)
		r.Post("/admin/drafts/{id}/reject", h.AdminDraftReject)
		r.Get("/admin/inventory", h.AdminInventory)
		r.Post("/admin/inventory/{variantID}", h.AdminInventoryUpdate)
		r.Get("/admin/orders", h.AdminOrders)
		r.Get("/admin/orders/{orderID}", h.AdminOrderDetail)
		r.Post("/admin/orders/{orderID}/status", h.AdminOrderUpdateStatus)
		r.Post("/admin/prescriptions/{id}/{action}", h.AdminVerifyPrescription)
	})
}

type PageData struct {
	User          *auth.User
	CartItemCount int
	Title         string
	Error         string
	Success       string
	Data          interface{}
}

func (h *Handler) errorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	h.render(w, r, "home", map[string]interface{}{
		"Error": message,
		"User":  h.currentUser(r),
	})
}

func (h *Handler) pageData(r *http.Request) PageData {
	user := h.currentUser(r)
	token := h.sessionToken(r)
	count, _ := h.cartStore.CartItemCount(h.userID(user), token)
	return PageData{User: user, CartItemCount: count}
}
