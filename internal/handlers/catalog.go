package handlers

import (
	"net/http"
	"strconv"

	"pharmacy/internal/catalog"

	"github.com/go-chi/chi/v5"
)

type homeData struct {
	PageData
	Categories []catalog.Category
	Result     *catalog.ProductListResult
	Query      string
	Category   string
}

func (h *Handler) HomePage(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	result, err := h.catalogStore.ListProducts(catalog.ProductListParams{
		CategorySlug: r.URL.Query().Get("category"),
		Search:       r.URL.Query().Get("q"),
		Page:         page,
		PerPage:      12,
	})
	if err != nil {
		h.log.Error("list products", "error", err)
		h.errorPage(w, r, 500, "Failed to load products")
		return
	}

	cats, err := h.catalogStore.Categories()
	if err != nil {
		h.log.Error("categories", "error", err)
	}

	data := homeData{
		PageData:   pd,
		Categories: cats,
		Result:     result,
		Query:      r.URL.Query().Get("q"),
		Category:   r.URL.Query().Get("category"),
	}
	data.Title = "Pharmacy"

	isHX := r.Header.Get("HX-Request") == "true"
	isBoosted := r.Header.Get("HX-Boosted") == "true"

	if isHX && !isBoosted {
		h.renderPartial(w, r, "product_grid", data)
		return
	}
	h.render(w, r, "home", data)
}

func (h *Handler) SearchResults(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	result, err := h.catalogStore.ListProducts(catalog.ProductListParams{
		Search:  r.URL.Query().Get("q"),
		Page:    page,
		PerPage: 12,
	})
	if err != nil {
		h.log.Error("search", "error", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	cats, _ := h.catalogStore.Categories()

	data := homeData{
		PageData:   pd,
		Categories: cats,
		Result:     result,
		Query:      r.URL.Query().Get("q"),
	}

	h.renderPartial(w, r, "product_grid", data)
}

func (h *Handler) ProductDetail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	pd := h.pageData(r)

	product, err := h.catalogStore.GetProductBySlug(slug)
	if err != nil {
		h.errorPage(w, r, 404, "Product not found")
		return
	}

	data := struct {
		PageData
		Product *catalog.ProductWithVariants
	}{pd, product}
	data.Title = product.Name

	h.render(w, r, "product", data)
}
