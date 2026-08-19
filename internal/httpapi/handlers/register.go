package handlers

import "github.com/gin-gonic/gin"

// RegisterRoutes installs every P6-04 domain group. Infrastructure routes and
// middleware remain owned by internal/httpapi; this function only wires the
// domain-facing surface.
func RegisterRoutes(router gin.IRouter, app *Application) {
	h := New(app)
	h.RegisterAuth(router)
	h.RegisterAccount(router)
	h.RegisterFriend(router)
	h.RegisterFarm(router)
	h.RegisterBag(router)
	h.RegisterMall(router)
	h.RegisterTask(router)
	h.RegisterActivity(router)
	h.RegisterIllustrated(router)
	h.RegisterCareer(router)
	h.RegisterAnalytics(router)
	h.RegisterSettings(router)
	h.RegisterSystem(router)
	h.RegisterUser(router)
	h.RegisterCard(router)
	h.RegisterLoginLog(router)
	h.RegisterYYB(router)
	h.RegisterCapture(router)
	h.RegisterQRLogin(router)
	h.RegisterProxy(router)
	h.RegisterPublicInfo(router)
	h.RegisterSeedShop(router)
	h.RegisterPetShop(router)
	h.RegisterDecorationShop(router)
	h.RegisterMysteryShop(router)
}

func (h *Handler) RegisterAll(router gin.IRouter) { RegisterRoutes(router, h.App) }
