package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) CartPage(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	user := h.currentUser(r)
	cart, err := h.cartStore.GetCart(h.userID(user), h.sessionToken(r))
	if err != nil {
		h.log.Error("get cart", "error", err)
	}

	data := struct {
		PageData
		Cart interface{}
	}{pd, cart}
	data.Title = "Shopping Cart"
	h.render(w, r, "cart", data)
}

func (h *Handler) CartAdd(w http.ResponseWriter, r *http.Request) {
	variantID, _ := strconv.ParseInt(r.FormValue("variant_id"), 10, 64)
	quantity := int32(1)
	if q, err := strconv.Atoi(r.FormValue("quantity")); err == nil && q > 0 {
		quantity = int32(q)
	}

	user := h.currentUser(r)
	err := h.cartStore.AddItem(h.userID(user), h.sessionToken(r), variantID, quantity)
	if err != nil {
		h.log.Error("add to cart", "error", err)
		http.Error(w, "Failed to add item", http.StatusInternalServerError)
		return
	}

	// If htmx request, return cart drawer partial
	if r.Header.Get("HX-Request") == "true" {
		h.CartDrawer(w, r)
		return
	}
	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}

func (h *Handler) CartUpdate(w http.ResponseWriter, r *http.Request) {
	itemID, _ := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
	quantity, _ := strconv.Atoi(r.FormValue("quantity"))

	if err := h.cartStore.UpdateQuantity(itemID, int32(quantity)); err != nil {
		h.log.Error("update cart", "error", err)
	}

	if r.Header.Get("HX-Request") == "true" {
		h.CartDrawer(w, r)
		return
	}
	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}

func (h *Handler) CartRemove(w http.ResponseWriter, r *http.Request) {
	itemID, _ := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)

	if err := h.cartStore.RemoveItem(itemID); err != nil {
		h.log.Error("remove from cart", "error", err)
	}

	if r.Header.Get("HX-Request") == "true" {
		h.CartDrawer(w, r)
		return
	}
	http.Redirect(w, r, "/cart", http.StatusSeeOther)
}

func (h *Handler) CartCount(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	count, _ := h.cartStore.CartItemCount(h.userID(user), h.sessionToken(r))
	pd := h.pageData(r)
	pd.CartItemCount = count
	h.renderPartial(w, r, "cart_badge", pd)
}

func (h *Handler) CartDrawer(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	cart, err := h.cartStore.GetCart(h.userID(user), h.sessionToken(r))
	if err != nil {
		cart = nil
	}

	pd := h.pageData(r)
	data := struct {
		PageData
		Cart interface{}
	}{pd, cart}
	h.renderPartial(w, r, "cart_drawer", data)
}
