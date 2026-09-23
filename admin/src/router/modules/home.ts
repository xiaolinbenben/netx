const Layout = () => import("@/layout/index.vue");

export default {
  path: "/",
  name: "Home",
  component: Layout,
  redirect: "/codes",
  meta: {
    icon: "ep/tickets",
    title: "NetX",
    rank: 0
  },
  children: [
    {
      path: "/codes",
      name: "Codes",
      component: () => import("@/views/codes/index.vue"),
      meta: {
        title: "兑换码",
        icon: "ep/tickets"
      }
    },
    {
      path: "/settings",
      name: "Settings",
      component: () => import("@/views/settings/index.vue"),
      meta: {
        title: "系统配置",
        icon: "ep/setting"
      }
    }
  ]
} satisfies RouteConfigsTable;
