package handlers

import (
	"net/http"
	"strconv"

	"pharmacy/internal/orders"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) CheckoutPage(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r)
	user := h.currentUser(r)
	cart, err := h.cartStore.GetCart(h.userID(user), h.sessionToken(r))
	if err != nil || len(cart.Items) == 0 {
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}

	data := struct {
		PageData
		Cart interface{}
	}{pd, cart}
	data.Title = "Checkout"
	h.render(w, r, "checkout", data)
}

func (h *Handler) CheckoutPost(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	cart, err := h.cartStore.GetCart(h.userID(user), h.sessionToken(r))
	if err != nil || len(cart.Items) == 0 {
		h.errorPage(w, r, 400, "Cart is empty")
		return
	}

	addressLine1 := r.FormValue("address_line1")
	addressCity := r.FormValue("address_city")
	addressState := r.FormValue("address_state")
	addressPhone := r.FormValue("address_phone")

	if addressLine1 == "" || addressCity == "" || addressState == "" {
		pd := h.pageData(r)
		pd.Error = "Please fill in all required address fields"
		data := struct {
			PageData
			Cart interface{}
		}{pd, cart}
		h.render(w, r, "checkout", data)
		return
	}

	var lineItems []orders.LineItem
	for _, item := range cart.Items {
		lineItems = append(lineItems, orders.LineItem{
			VariantID:   item.VariantID,
			ProductName: item.ProductName + " - " + item.Strength + " " + item.PackSize,
			PriceCents:  item.PriceCents,
			Quantity:    item.Quantity,
		})
	}

	createdOrder, err := h.orderStore.Checkout(orders.CheckoutInput{
		UserID:       user.ID,
		CartID:       cart.ID,
		LineItems:    lineItems,
		AddressLine1: addressLine1,
		AddressCity:  addressCity,
		AddressState: addressState,
		AddressPhone: addressPhone,
	})
	if err != nil {
		h.log.Error("checkout", "error", err)
		if err == orders.ErrOutOfStock || containsOutOfStock(err) {
			h.errorPage(w, r, 400, "Some items are out of stock")
		} else {
			h.errorPage(w, r, 500, "Checkout failed")
		}
		return
	}

	// Check if any items require prescription upload
	needsPrescription := false
	for _, item := range cart.Items {
		if item.RequiresPrescription {
			needsPrescription = true
			break
		}
	}
	if needsPrescription {
		http.Redirect(w, r, "/checkout/pay/" + strconv.FormatInt(createdOrder.ID, 10) + "?prescription=1", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/checkout/pay/"+strconv.FormatInt(createdOrder.ID, 10), http.StatusSeeOther)
}

func containsOutOfStock(err error) bool {
	return err != nil && (err == orders.ErrOutOfStock || containsOutOfStockRecursive(err))
}

func containsOutOfStockRecursive(err error) bool {
	type causer interface {
		Cause() error
	}
	if c, ok := err.(causer); ok {
		return containsOutOfStock(c.Cause())
	}
	return false
}

func (h *Handler) PaymentPage(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.ParseInt(chi.URLParam(r, "orderID"), 10, 64)
	pd := h.pageData(r)

	order, err := h.orderStore.GetOrder(orderID)
	if err != nil {
		h.errorPage(w, r, 404, "Order not found")
		return
	}

	items, err := h.orderStore.GetOrderItems(orderID)
	if err != nil {
		items = nil
	}

	needsPrescription := r.URL.Query().Get("prescription") == "1"
	var prescriptions interface{}
	if needsPrescription {
		ps, _ := h.orderStore.GetPrescriptionsByOrder(orderID)
		prescriptions = ps
	}

	data := struct {
		PageData
		Order             *orders.Order
		Items             []orders.OrderItem
		NeedsPrescription bool
		Prescriptions     interface{}
	}{pd, order, items, needsPrescription, prescriptions}
	data.Title = "Payment"

	h.render(w, r, "order", data)
}

func (h *Handler) PaymentPost(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.ParseInt(chi.URLParam(r, "orderID"), 10, 64)

	order, err := h.orderStore.GetOrder(orderID)
	if err != nil {
		h.errorPage(w, r, 404, "Order not found")
		return
	}

	// Handle prescription upload
	if r.FormValue("prescription") == "1" {
		// Stub: just create a prescription record
		_, err := h.orderStore.CreatePrescription(orderID, "uploads/prescriptions/stub.jpg")
		if err != nil {
			h.log.Error("create prescription", "error", err)
		}
		http.Redirect(w, r, "/orders/"+chi.URLParam(r, "orderID"), http.StatusSeeOther)
		return
	}

	// Initiate payment via stub provider
	ref, err := h.paymentProv.InitiateCheckout(order.TotalCents, orderID)
	if err != nil {
		h.log.Error("initiate payment", "error", err)
		h.errorPage(w, r, 500, "Payment failed")
		return
	}

	// Stub: automatically verify the payment
	success, err := h.paymentProv.VerifyWebhook(ref)
	if err != nil {
		h.log.Error("verify payment", "error", err)
	}

	if success {
		h.orderStore.UpdateOrderStatus(orderID, "paid")
	}

	http.Redirect(w, r, "/orders/"+strconv.FormatInt(orderID, 10), http.StatusSeeOther)
}

func (h *Handler) OrderHistory(w http.ResponseWriter, r *http.Request) {
	user := h.currentUser(r)
	pd := h.pageData(r)

	ordersList, err := h.orderStore.ListOrdersByUser(user.ID)
	if err != nil {
		ordersList = nil
	}

	data := struct {
		PageData
		Orders []orders.Order
	}{pd, ordersList}
	data.Title = "My Orders"
	h.render(w, r, "orders", data)
}

func (h *Handler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.ParseInt(chi.URLParam(r, "orderID"), 10, 64)
	pd := h.pageData(r)

	order, err := h.orderStore.GetOrder(orderID)
	if err != nil {
		h.errorPage(w, r, 404, "Order not found")
		return
	}

	items, err := h.orderStore.GetOrderItems(orderID)
	if err != nil {
		items = nil
	}

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
