package handlers

import (
	"net/http"
	"strconv"

	"pharmacy/internal/admin"
	"pharmacy/internal/catalog"
	"pharmacy/internal/orders"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	pd.Title = "Admin Dashboard"

	drafts, _ := h.adminStore().ListDrafts("pending_review")
	allOrders, _ := h.orderStore.AllOrders("")

	data := struct {
		PageData
		Drafts      []admin.ProductDraft
		Orders      []orders.Order
		DraftCount  int
		OrderCount  int
	}{pd, drafts, allOrders, len(drafts), len(allOrders)}

	h.render(w, r, "admin_drafts", data)
}

func (h *Handler) adminStore() *admin.Store {
	return admin.NewStore(h.orderStore.DB())
}

func (h *Handler) AdminDrafts(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	pd.Title = "Product Draft Review"

	drafts, err := h.adminStore().ListDrafts("")
	if err != nil {
		drafts = nil
	}
	allOrders, _ := h.orderStore.AllOrders("")

	data := struct {
		PageData
		Drafts     []admin.ProductDraft
		DraftCount int
		OrderCount int
	}{pd, drafts, len(drafts), len(allOrders)}
	h.render(w, r, "admin_drafts", data)
}

func (h *Handler) AdminDraftDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pd := h.pageData(r)

	draft, err := h.adminStore().GetDraft(id)
	if err != nil {
		h.errorPage(w, r, 404, "Draft not found")
		return
	}

	cats, _ := h.catalogStore.Categories()

	data := struct {
		PageData
		Draft      *admin.ProductDraft
		Categories []catalog.Category
	}{pd, draft, cats}
	data.Title = "Review Draft"
	h.render(w, r, "admin_draft", data)
}

func (h *Handler) AdminDraftApprove(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	user := h.currentUser(r)

	fields := admin.SuggestedProduct{
		Name:       r.FormValue("name"),
		Brand:      r.FormValue("brand"),
		Slug:       r.FormValue("slug"),
		Strength:   r.FormValue("strength"),
		PackSize:   r.FormValue("pack_size"),
		SKU:        r.FormValue("sku"),
		Description: r.FormValue("description"),
	}
	cid, _ := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	fields.CategoryID = cid
	pc, _ := strconv.Atoi(r.FormValue("price_cents"))
	fields.PriceCents = int32(pc)
	sq, _ := strconv.Atoi(r.FormValue("stock_qty"))
	fields.StockQty = int32(sq)
	fields.RequiresPrescription = r.FormValue("requires_prescription") == "on"

	if err := h.adminStore().ApproveDraft(id, user.ID, fields); err != nil {
		h.log.Error("approve draft", "error", err)
		http.Error(w, "Failed to approve", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/drafts", http.StatusSeeOther)
}

func (h *Handler) AdminDraftReject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	user := h.currentUser(r)

	if err := h.adminStore().RejectDraft(id, user.ID); err != nil {
		h.log.Error("reject draft", "error", err)
		http.Error(w, "Failed to reject", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/drafts", http.StatusSeeOther)
}

func (h *Handler) AdminInventory(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	pd.Title = "Inventory Management"

	result, err := h.catalogStore.ListProducts(catalog.ProductListParams{Page: 1, PerPage: 100})
	if err != nil {
		result = &catalog.ProductListResult{}
	}

	data := struct {
		PageData
		Result *catalog.ProductListResult
	}{pd, result}
	h.render(w, r, "admin_inventory", data)
}

func (h *Handler) AdminInventoryUpdate(w http.ResponseWriter, r *http.Request) {
	variantID, _ := strconv.ParseInt(chi.URLParam(r, "variantID"), 10, 64)
	price, _ := strconv.Atoi(r.FormValue("price_cents"))
	stock, _ := strconv.Atoi(r.FormValue("stock_qty"))

	if price > 0 {
		h.catalogStore.UpdateVariantPrice(variantID, int32(price))
	}
	if stock >= 0 {
		h.catalogStore.UpdateVariantStock(variantID, int32(stock))
	}

	http.Redirect(w, r, "/admin/inventory", http.StatusSeeOther)
}

func (h *Handler) AdminOrders(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	pd.Title = "Order Management"

	status := r.URL.Query().Get("status")
	allOrders, err := h.orderStore.AllOrders(status)
	if err != nil {
		allOrders = nil
	}

	data := struct {
		PageData
		Orders []orders.Order
		Filter string
	}{pd, allOrders, status}
	h.render(w, r, "admin_orders", data)
}

func (h *Handler) AdminOrderDetail(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.ParseInt(chi.URLParam(r, "orderID"), 10, 64)
	pd := h.pageData(r)

	order, err := h.orderStore.GetOrder(orderID)
	if err != nil {
		h.errorPage(w, r, 404, "Order not found")
		return
	}

	items, _ := h.orderStore.GetOrderItems(orderID)
	prescriptions, _ := h.orderStore.GetPrescriptionsByOrder(orderID)

	data := struct {
		PageData
		Order             *orders.Order
		Items             []orders.OrderItem
		NeedsPrescription bool
		Prescriptions     []orders.Prescription
	}{pd, order, items, false, prescriptions}
	data.Title = "Order #" + strconv.FormatInt(orderID, 10)
	h.render(w, r, "order", data)
}

func (h *Handler) AdminOrderUpdateStatus(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.ParseInt(chi.URLParam(r, "orderID"), 10, 64)
	status := r.FormValue("status")

	if err := h.orderStore.UpdateOrderStatus(orderID, status); err != nil {
		h.log.Error("update order status", "error", err)
	}
	http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
}

func (h *Handler) AdminVerifyPrescription(w http.ResponseWriter, r *http.Request) {
	prescriptionID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	action := chi.URLParam(r, "action")
	user := h.currentUser(r)

	status := "rejected"
	if action == "approve" {
		status = "approved"
	}

	if err := h.orderStore.VerifyPrescription(prescriptionID, user.ID, status); err != nil {
		h.log.Error("verify prescription", "error", err)
	}

	http.Redirect(w, r, "/admin/orders", http.StatusSeeOther)
}
