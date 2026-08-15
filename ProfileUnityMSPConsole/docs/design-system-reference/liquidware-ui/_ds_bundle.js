/* @ds-bundle: {"format":3,"namespace":"LiquidwareUI_c7956d","components":[],"sourceHashes":{"ui_kits/stratusphere-ux/DetailedViews.jsx":"9a7e80332336","ui_kits/stratusphere-ux/IndividualViews.jsx":"4e985efb88ec","ui_kits/stratusphere-ux/LoginScreen.jsx":"662f52575805","ui_kits/stratusphere-ux/Overview.jsx":"8c6c0c2e3ab6","ui_kits/stratusphere-ux/Shell.jsx":"3adebfe8f5da","ui_kits/stratusphere-ux/Widgets.jsx":"9bcb0240a91e","ui_kits/stratusphere-ux/data.jsx":"f9110fb14934"},"inlinedExternals":[],"unexposedExports":[]} */

(() => {

const __ds_ns = (window.LiquidwareUI_c7956d = window.LiquidwareUI_c7956d || {});

const __ds_scope = {};

(__ds_ns.__errors = __ds_ns.__errors || []);

// ui_kits/stratusphere-ux/DetailedViews.jsx
try { (() => {
/* Detailed Views — the heatmap data table. Each metric cell is shaded by
   severity (exact s-colors + letter grades from the product's v1 API). */

function DVCell({
  col,
  value
}) {
  let text = value,
    bg = null,
    fg = null;
  if (Array.isArray(value)) {
    text = value[0];
    bg = value[1];
    fg = value[2] || '#000';
  }
  const cls = 'dv-cell ' + (col.align === 'r' ? 'rgt' : col.align === 'c' ? 'ctr' : '');
  if (col.grade) {
    return /*#__PURE__*/React.createElement("td", {
      className: cls
    }, /*#__PURE__*/React.createElement("span", {
      className: "dv-grade",
      style: {
        background: bg,
        color: fg
      }
    }, text));
  }
  return /*#__PURE__*/React.createElement("td", {
    className: cls,
    style: bg ? {
      background: bg,
      color: fg
    } : null
  }, window.fmt(text));
}
function DetailedViews() {
  const tabs = [{
    label: 'Users & Machines - User - Users',
    icon: 'pi pi-users',
    active: true
  }, {
    label: 'Users & Machines - User - User,…',
    icon: 'pi pi-users'
  }, {
    label: 'April Users',
    mi: 'history_edu'
  }, {
    label: 'Browser - Domain',
    mi: 'table_view'
  }];
  const [active, setActive] = React.useState(tabs[0].label);
  return /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("div", {
    className: "view-tabs"
  }, tabs.map(t => /*#__PURE__*/React.createElement("div", {
    key: t.label,
    className: 'view-tab' + (active === t.label ? ' active' : ''),
    onClick: () => setActive(t.label)
  }, t.mi ? /*#__PURE__*/React.createElement("span", {
    className: "material-icons"
  }, t.mi) : /*#__PURE__*/React.createElement("i", {
    className: t.icon
  }), " ", t.label))), /*#__PURE__*/React.createElement("div", {
    className: "controls"
  }, /*#__PURE__*/React.createElement("div", {
    className: "controls-left"
  }, /*#__PURE__*/React.createElement("div", {
    className: "daterange"
  }, /*#__PURE__*/React.createElement("span", null, "5/3/26 - 5/9/26"), /*#__PURE__*/React.createElement("span", {
    className: "dr-cal"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-calendar"
  }))), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-table"
  }), " Columns"), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-filter"
  }), " Filter & API Options"), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-refresh"
  }))), /*#__PURE__*/React.createElement("div", {
    className: "controls-right"
  }, /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-undo"
  })), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-refresh",
    style: {
      transform: 'scaleX(-1)'
    }
  })), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sliders-h"
  })))), /*#__PURE__*/React.createElement("div", {
    className: "dv-wrap"
  }, /*#__PURE__*/React.createElement("div", {
    className: "dv-scroll"
  }, /*#__PURE__*/React.createElement("table", {
    className: "dv-table"
  }, /*#__PURE__*/React.createElement("thead", null, /*#__PURE__*/React.createElement("tr", null, window.DV_COLS.map(c => /*#__PURE__*/React.createElement("th", {
    key: c.k,
    className: c.align === 'r' ? 'rgt' : c.align === 'c' ? 'ctr' : ''
  }, c.label, " ", /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sort-alt"
  }))))), /*#__PURE__*/React.createElement("tbody", null, window.DV_ROWS.map((row, i) => /*#__PURE__*/React.createElement("tr", {
    key: i
  }, window.DV_COLS.map(c => /*#__PURE__*/React.createElement(DVCell, {
    key: c.k,
    col: c,
    value: row[c.k]
  }))))))), /*#__PURE__*/React.createElement("div", {
    className: "dv-foot"
  }, /*#__PURE__*/React.createElement("span", {
    className: "lwl-muted"
  }, "Last Updated: 3 weeks 3 days ago"), /*#__PURE__*/React.createElement("div", {
    className: "dv-pager"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-angle-double-left"
  }), /*#__PURE__*/React.createElement("i", {
    className: "pi pi-angle-left"
  }), /*#__PURE__*/React.createElement("span", {
    className: "dv-page"
  }, "1"), /*#__PURE__*/React.createElement("i", {
    className: "pi pi-angle-right"
  }), /*#__PURE__*/React.createElement("i", {
    className: "pi pi-angle-double-right"
  }), /*#__PURE__*/React.createElement("span", {
    className: "dv-pagesize"
  }, "50 ", /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chevron-down"
  }))))));
}
window.DetailedViews = DetailedViews;
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/DetailedViews.jsx", error: String((e && e.message) || e) }); }

// ui_kits/stratusphere-ux/IndividualViews.jsx
try { (() => {
/* Individual Views — per-machine/user detail dashboard with view tabs,
   filter controls, and a grid of metric cards (some with area sparklines). */

function Sparkline({
  kind
}) {
  // simple inline SVG area chart; shapes vary by kind
  const shapes = {
    'area-flat': '0,30 12,26 24,28 36,24 48,25 60,22 72,23 84,20 96,18 108,8 120,6',
    'area-spiky': '0,34 12,30 24,18 36,24 48,12 60,20 72,10 84,22 96,16 108,26 120,30',
    'area-rise': '0,32 12,31 24,30 36,28 48,29 60,26 72,24 84,22 96,20 108,16 120,12',
    'area-peak': '0,34 12,33 24,32 36,30 48,22 60,6 72,18 84,28 96,24 108,30 120,33'
  };
  if (kind === 'bar-block') {
    return /*#__PURE__*/React.createElement("svg", {
      className: "spark",
      viewBox: "0 0 120 40",
      preserveAspectRatio: "none"
    }, /*#__PURE__*/React.createElement("rect", {
      x: "40",
      y: "2",
      width: "60",
      height: "38",
      className: "spark-fill"
    }));
  }
  const pts = shapes[kind] || shapes['area-flat'];
  return /*#__PURE__*/React.createElement("svg", {
    className: "spark",
    viewBox: "0 0 120 40",
    preserveAspectRatio: "none"
  }, /*#__PURE__*/React.createElement("polygon", {
    className: "spark-fill",
    points: `0,40 ${pts} 120,40`
  }), /*#__PURE__*/React.createElement("polyline", {
    className: "spark-line",
    points: pts,
    fill: "none"
  }));
}
function IVCardIcon({
  c
}) {
  if (c.mi) return /*#__PURE__*/React.createElement("span", {
    className: "material-icons"
  }, c.mi);
  return /*#__PURE__*/React.createElement("i", {
    className: c.icon
  });
}
function IVCard({
  c
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "card iv-card"
  }, /*#__PURE__*/React.createElement("div", {
    className: "iv-head"
  }, /*#__PURE__*/React.createElement("span", {
    className: 'iv-title' + (c.link ? ' link' : '')
  }, /*#__PURE__*/React.createElement(IVCardIcon, {
    c: c
  }), " ", c.title), (c.link || c.chart) && /*#__PURE__*/React.createElement("i", {
    className: "pi pi-ellipsis-h ov-more"
  })), c.empty ? /*#__PURE__*/React.createElement("div", {
    className: "iv-empty"
  }) : c.stub ? /*#__PURE__*/React.createElement("div", {
    className: "iv-stub lwl-muted"
  }, c.stub) : /*#__PURE__*/React.createElement(React.Fragment, null, c.big && /*#__PURE__*/React.createElement("div", {
    className: "iv-big"
  }, c.big, c.id !== 'machine' && c.id !== 'os' && c.id !== 'user' && /*#__PURE__*/React.createElement("span", {
    className: "iv-unit"
  })), c.sub && /*#__PURE__*/React.createElement("div", {
    className: "iv-sub"
  }, c.sub), c.chart && /*#__PURE__*/React.createElement(Sparkline, {
    kind: c.chart
  }), /*#__PURE__*/React.createElement("div", {
    className: "iv-rows"
  }, c.rows && c.rows.map(([k, v], i) => /*#__PURE__*/React.createElement("div", {
    key: i,
    className: "iv-row"
  }, /*#__PURE__*/React.createElement("span", {
    className: "iv-k"
  }, k, ":"), " ", /*#__PURE__*/React.createElement("b", null, v))))));
}
function IndividualViews() {
  const [tab, setTab] = React.useState('Summary');
  const [viewTab, setViewTab] = React.useState('HAL-9000 / John Doe');
  const viewTabs = ['HAL-9000 / John Doe', 'Skynet'];
  return /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("div", {
    className: "view-tabs"
  }, viewTabs.map(v => /*#__PURE__*/React.createElement("div", {
    key: v,
    className: 'view-tab' + (viewTab === v ? ' active' : ''),
    onClick: () => setViewTab(v)
  }, /*#__PURE__*/React.createElement("span", {
    className: "material-icons"
  }, "grid_view"), " ", v)), /*#__PURE__*/React.createElement("div", {
    className: "view-tab newtab"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-plus"
  }), " New Tab")), /*#__PURE__*/React.createElement("div", {
    className: "controls"
  }, /*#__PURE__*/React.createElement("div", {
    className: "controls-left"
  }, /*#__PURE__*/React.createElement("div", {
    className: "daterange"
  }, /*#__PURE__*/React.createElement("span", null, "6/4/26"), /*#__PURE__*/React.createElement("span", {
    className: "dr-cal"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-calendar"
  }))), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-primary"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-filter"
  }), " Filters ", /*#__PURE__*/React.createElement("span", {
    className: "badge-count"
  }, "1")), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-text-link"
  }, "Clear filters"), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-refresh"
  }))), /*#__PURE__*/React.createElement("div", {
    className: "controls-right"
  }, /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sliders-h"
  })))), /*#__PURE__*/React.createElement("div", {
    className: "tabbar"
  }, window.IV_TABS.map(t => /*#__PURE__*/React.createElement("div", {
    key: t,
    className: 'tab' + (tab === t ? ' active' : ''),
    onClick: () => setTab(t)
  }, t))), tab === 'Summary' ? /*#__PURE__*/React.createElement("div", {
    className: "ov-scroll"
  }, /*#__PURE__*/React.createElement("div", {
    className: "iv-grid"
  }, window.IV_CARDS.map(c => /*#__PURE__*/React.createElement(IVCard, {
    key: c.id,
    c: c
  })))) : /*#__PURE__*/React.createElement("div", {
    className: "stub"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chart-line"
  }), /*#__PURE__*/React.createElement("h2", null, tab), /*#__PURE__*/React.createElement("p", null, "The ", tab, " tab drills into ", tab.toLowerCase(), " metrics for this machine. Select ", /*#__PURE__*/React.createElement("b", null, "Summary"), " for the populated view.")));
}
window.IndividualViews = IndividualViews;
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/IndividualViews.jsx", error: String((e && e.message) || e) }); }

// ui_kits/stratusphere-ux/LoginScreen.jsx
try { (() => {
/* Login screen — recreation of login-page.html.
   Frost form panel over the primary-950 + hex background. */
function LoginScreen({
  onLogin
}) {
  const [domain, setDomain] = React.useState(window.DOMAINS[0]);
  const [user, setUser] = React.useState('');
  const [pass, setPass] = React.useState('');
  const [show, setShow] = React.useState(false);
  const [remember, setRemember] = React.useState(true);
  const [busy, setBusy] = React.useState(false);
  const valid = user.trim() && pass.trim();
  const submit = e => {
    e.preventDefault();
    if (!valid) return;
    setBusy(true);
    setTimeout(() => onLogin({
      user: user.trim()
    }), 850);
  };
  return /*#__PURE__*/React.createElement("main", {
    className: "login"
  }, /*#__PURE__*/React.createElement("form", {
    className: "login-form",
    onSubmit: submit
  }, /*#__PURE__*/React.createElement("div", {
    className: "login-logo"
  }, /*#__PURE__*/React.createElement("img", {
    src: "../../assets/logo-primary-light.svg",
    alt: "Liquidware Labs, Inc."
  })), /*#__PURE__*/React.createElement("label", {
    className: "login-select"
  }, /*#__PURE__*/React.createElement("select", {
    value: domain,
    onChange: e => setDomain(e.target.value)
  }, window.DOMAINS.map(d => /*#__PURE__*/React.createElement("option", {
    key: d,
    value: d
  }, d))), /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chevron-down"
  })), /*#__PURE__*/React.createElement("div", {
    className: "login-field"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-user lead"
  }), /*#__PURE__*/React.createElement("input", {
    type: "text",
    placeholder: "Username",
    value: user,
    autoComplete: "off",
    onChange: e => setUser(e.target.value)
  })), /*#__PURE__*/React.createElement("div", {
    className: "login-field"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-lock lead"
  }), /*#__PURE__*/React.createElement("input", {
    type: show ? 'text' : 'password',
    placeholder: "Password",
    value: pass,
    autoComplete: "off",
    onChange: e => setPass(e.target.value)
  }), /*#__PURE__*/React.createElement("i", {
    className: 'pi trail ' + (show ? 'pi-eye-slash' : 'pi-eye'),
    title: "Toggle Visibility",
    onClick: () => setShow(s => !s)
  })), /*#__PURE__*/React.createElement("label", {
    className: "login-remember"
  }, /*#__PURE__*/React.createElement("span", {
    className: 'cb ' + (remember ? 'on' : ''),
    onClick: () => setRemember(r => !r)
  }, remember && /*#__PURE__*/React.createElement("i", {
    className: "pi pi-check"
  })), "Remember me"), /*#__PURE__*/React.createElement("div", {
    className: "login-actions"
  }, /*#__PURE__*/React.createElement("button", {
    type: "button",
    className: "btn btn-outlined login-license"
  }, "License info"), /*#__PURE__*/React.createElement("button", {
    type: "submit",
    className: "btn btn-primary",
    disabled: !valid || busy
  }, busy && /*#__PURE__*/React.createElement("i", {
    className: "pi pi-spin pi-spinner"
  }), /*#__PURE__*/React.createElement("span", null, "Sign In")))), /*#__PURE__*/React.createElement("p", {
    className: "login-copy"
  }, "Copyright \xA9 2010-", new Date().getFullYear(), " Liquidware Labs, Inc"));
}
window.LoginScreen = LoginScreen;
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/LoginScreen.jsx", error: String((e && e.message) || e) }); }

// ui_kits/stratusphere-ux/Overview.jsx
try { (() => {
/* Overview dashboard page + a generic stub for not-yet-built screens. */

function Overview() {
  const [tab, setTab] = React.useState('Summary');
  const [tick, setTick] = React.useState(0);
  return /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("div", {
    className: "tabbar"
  }, window.OVERVIEW_TABS.map(t => /*#__PURE__*/React.createElement("div", {
    key: t.label,
    className: 'tab' + (tab === t.label ? ' active' : ''),
    onClick: () => setTab(t.label)
  }, /*#__PURE__*/React.createElement("i", {
    className: t.icon
  }), " ", t.label))), /*#__PURE__*/React.createElement(OverviewControls, {
    onRefresh: () => setTick(t => t + 1)
  }), tab === 'Summary' ? /*#__PURE__*/React.createElement("div", {
    className: "ov-scroll",
    key: tick
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-observed"
  }, window.OBSERVED.map(t => /*#__PURE__*/React.createElement(ObservedBar, {
    key: t.id,
    tile: t
  }))), /*#__PURE__*/React.createElement("div", {
    className: "ov-grid"
  }, window.OV_WIDGETS.map(w => /*#__PURE__*/React.createElement(OverviewWidget, {
    key: w.id,
    w: w
  })))) : /*#__PURE__*/React.createElement("div", {
    className: "stub"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chart-bar"
  }), /*#__PURE__*/React.createElement("h2", null, tab), /*#__PURE__*/React.createElement("p", null, "This Overview tab follows the same card grid and Good / Fair / Poor language shown on ", /*#__PURE__*/React.createElement("b", null, "Summary"), ". Select ", /*#__PURE__*/React.createElement("b", null, "Summary"), " to see the populated dashboard.")));
}
function Stub({
  title,
  icon,
  mi
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "stub"
  }, mi ? /*#__PURE__*/React.createElement("span", {
    className: "material-icons",
    style: {
      fontSize: 44
    }
  }, mi) : /*#__PURE__*/React.createElement("i", {
    className: icon || 'pi pi-compass'
  }), /*#__PURE__*/React.createElement("h2", null, title), /*#__PURE__*/React.createElement("p", null, "Part of the Stratusphere UX console. This UI kit fully builds out ", /*#__PURE__*/React.createElement("b", null, "Overview"), ", ", /*#__PURE__*/React.createElement("b", null, "Individual Views"), ", and ", /*#__PURE__*/React.createElement("b", null, "Detailed Views"), "; other areas share the same shell, controls, and GFP language."));
}
Object.assign(window, {
  Overview,
  Stub
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/Overview.jsx", error: String((e && e.message) || e) }); }

// ui_kits/stratusphere-ux/Shell.jsx
try { (() => {
/* Application header, side navigation, breadcrumbs — the app shell chrome.
   Mirrors application-header.html, application-side-navigation.ts, breadcrumbs. */

function AppHeader({
  onToggleNav,
  navPinned,
  user,
  dark,
  onToggleDark,
  onLogout
}) {
  const [focus, setFocus] = React.useState(false);
  const [q, setQ] = React.useState('');
  const [menu, setMenu] = React.useState(false);
  const initials = (user.user || 'JS').slice(0, 2).toUpperCase();
  return /*#__PURE__*/React.createElement("header", {
    className: "hdr"
  }, /*#__PURE__*/React.createElement("div", {
    className: "hdr-left"
  }, /*#__PURE__*/React.createElement("button", {
    className: "hdr-iconbtn",
    title: "Toggle Menu",
    onClick: onToggleNav
  }, /*#__PURE__*/React.createElement("i", {
    className: 'pi ' + (navPinned ? 'pi-window-minimize' : 'pi-window-maximize')
  })), /*#__PURE__*/React.createElement("a", {
    className: "hdr-logo",
    title: "Stratusphere UX Home page"
  }, /*#__PURE__*/React.createElement("img", {
    src: "../../assets/logo-primary-light.svg",
    alt: "Stratusphere UX"
  })), /*#__PURE__*/React.createElement("span", {
    className: "hdr-ver"
  }, "6.7.1-1")), /*#__PURE__*/React.createElement("div", {
    className: "hdr-search"
  }, /*#__PURE__*/React.createElement("div", {
    className: 'field' + (focus ? ' focus' : '')
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-search"
  }), /*#__PURE__*/React.createElement("input", {
    value: q,
    placeholder: "Search",
    spellCheck: "false",
    onFocus: () => setFocus(true),
    onBlur: () => setFocus(false),
    onChange: e => setQ(e.target.value)
  }), q && /*#__PURE__*/React.createElement("i", {
    className: "pi pi-times",
    style: {
      cursor: 'pointer'
    },
    onClick: () => setQ('')
  }))), /*#__PURE__*/React.createElement("div", {
    className: "hdr-right"
  }, /*#__PURE__*/React.createElement("button", {
    className: "hdr-iconbtn",
    title: "help"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-question-circle"
  })), /*#__PURE__*/React.createElement("div", {
    style: {
      position: 'relative'
    }
  }, /*#__PURE__*/React.createElement("span", {
    className: "avatar",
    title: user.user,
    onClick: () => setMenu(m => !m)
  }, initials), menu && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement("div", {
    style: {
      position: 'fixed',
      inset: 0,
      zIndex: 40
    },
    onClick: () => setMenu(false)
  }), /*#__PURE__*/React.createElement("div", {
    className: "menu",
    style: {
      right: 0,
      top: 38
    }
  }, /*#__PURE__*/React.createElement("div", {
    className: "menu-item"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-cog"
  }), " Settings"), /*#__PURE__*/React.createElement("div", {
    className: "menu-item",
    onClick: () => {
      onToggleDark();
      setMenu(false);
    }
  }, /*#__PURE__*/React.createElement("i", {
    className: 'pi ' + (dark ? 'pi-moon' : 'pi-sun')
  }), " Toggle Visual Mode"), /*#__PURE__*/React.createElement("div", {
    className: "menu-item",
    onClick: onLogout
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sign-out"
  }), " Logout"))))));
}
function NavIcon({
  item
}) {
  if (item.mi) return /*#__PURE__*/React.createElement("span", {
    className: "material-icons"
  }, item.mi);
  return /*#__PURE__*/React.createElement("i", {
    className: item.icon
  });
}
function SideNav({
  active,
  onSelect,
  collapsed
}) {
  return /*#__PURE__*/React.createElement("nav", {
    className: 'nav' + (collapsed ? ' collapsed' : ''),
    "aria-label": "application navigation"
  }, window.NAV.map((group, gi) => /*#__PURE__*/React.createElement("div", {
    className: "nav-group",
    key: gi
  }, group.label && /*#__PURE__*/React.createElement("div", {
    className: "nav-label"
  }, group.label), group.items.map(item => /*#__PURE__*/React.createElement("div", {
    key: item.key,
    className: 'nav-item' + (active === item.key ? ' active' : '') + (item.disabled ? ' disabled' : ''),
    onClick: () => !item.disabled && onSelect(item.key)
  }, /*#__PURE__*/React.createElement(NavIcon, {
    item: item
  }), /*#__PURE__*/React.createElement("span", null, item.label), item.chev && /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chevron-right chev"
  }))))));
}
function CrumbIcon({
  icon,
  mi
}) {
  if (mi) return /*#__PURE__*/React.createElement("span", {
    className: "material-icons"
  }, mi);
  if (icon) return /*#__PURE__*/React.createElement("i", {
    className: icon
  });
  return null;
}
function Breadcrumbs({
  trail
}) {
  return /*#__PURE__*/React.createElement("nav", {
    className: "crumbs",
    "aria-label": "Breadcrumb navigation"
  }, /*#__PURE__*/React.createElement("a", {
    className: "crumb-home"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-home"
  })), trail.map((t, i) => /*#__PURE__*/React.createElement(React.Fragment, {
    key: i
  }, /*#__PURE__*/React.createElement("span", {
    className: "sep"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-angle-right"
  })), i === trail.length - 1 ? /*#__PURE__*/React.createElement("span", {
    className: "cur"
  }, /*#__PURE__*/React.createElement(CrumbIcon, {
    icon: t.icon,
    mi: t.mi
  }), " ", t.label) : /*#__PURE__*/React.createElement("a", null, /*#__PURE__*/React.createElement(CrumbIcon, {
    icon: t.icon,
    mi: t.mi
  }), " ", t.label))));
}
Object.assign(window, {
  AppHeader,
  SideNav,
  Breadcrumbs
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/Shell.jsx", error: String((e && e.message) || e) }); }

// ui_kits/stratusphere-ux/Widgets.jsx
try { (() => {
/* Overview dashboard widgets — modeled on the real product. */

function WIcon({
  w
}) {
  if (w.mi) return /*#__PURE__*/React.createElement("span", {
    className: "material-icons"
  }, w.mi);
  return /*#__PURE__*/React.createElement("i", {
    className: w.icon
  });
}
function ObservedBar({
  tile
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "card observed"
  }, /*#__PURE__*/React.createElement("span", {
    className: "observed-label"
  }, /*#__PURE__*/React.createElement("i", {
    className: tile.icon
  }), " ", tile.label), /*#__PURE__*/React.createElement("span", {
    className: "observed-val"
  }, window.fmt(tile.amount), /*#__PURE__*/React.createElement("span", {
    className: "sep"
  }, "/", window.fmt(tile.total))));
}
function Dropdown({
  label
}) {
  return /*#__PURE__*/React.createElement("span", {
    className: "ov-dd"
  }, label, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chevron-down"
  }));
}
function WidgetShell({
  w,
  children
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "card ov-widget"
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-head"
  }, /*#__PURE__*/React.createElement("span", {
    className: 'ov-title' + (w.link ? ' link' : '')
  }, /*#__PURE__*/React.createElement(WIcon, {
    w: w
  }), " ", /*#__PURE__*/React.createElement("span", {
    className: "ov-title-text"
  }, w.title)), /*#__PURE__*/React.createElement("i", {
    className: "pi pi-ellipsis-h ov-more",
    title: "more"
  })), /*#__PURE__*/React.createElement("div", {
    className: "ov-body"
  }, children));
}
function Footer({
  footer
}) {
  if (!footer) return null;
  return /*#__PURE__*/React.createElement("div", {
    className: "ov-footer"
  }, footer.map(([label, val, color], i) => /*#__PURE__*/React.createElement("span", {
    key: i,
    className: "ov-foot"
  }, /*#__PURE__*/React.createElement("span", {
    className: "ov-foot-l"
  }, label, ":"), " ", /*#__PURE__*/React.createElement("b", {
    style: {
      color
    }
  }, val))));
}
function Legend({
  legend,
  active,
  setActive
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "ov-legend"
  }, legend.map(([label, val, color], i) => /*#__PURE__*/React.createElement("div", {
    key: i,
    className: 'ov-legrow' + (active === i ? ' on' : ''),
    onMouseEnter: () => setActive && setActive(i),
    onMouseLeave: () => setActive && setActive(null)
  }, /*#__PURE__*/React.createElement("span", {
    className: "ov-leglabel",
    style: {
      color
    }
  }, label), /*#__PURE__*/React.createElement("span", {
    className: "ov-legval",
    style: {
      color
    }
  }, val))));
}
function DonutWidget({
  w
}) {
  const [active, setActive] = React.useState(null);
  const vals = w.mix || w.legend.map(l => l[1]);
  const total = vals.reduce((a, b) => a + b, 0) || 1;
  let acc = 0;
  const stops = w.legend.map((l, i) => {
    const c = l[2];
    const start = acc / total * 100;
    acc += vals[i];
    const end = acc / total * 100;
    const dim = active != null && active !== i;
    return `${dim ? '#d4d4d8' : c} ${start}% ${end}%`;
  }).join(', ');
  // when everything is 0 show a full grey ring
  const ring = total === 0 ? '#d4d4d8 0 100%' : stops;
  return /*#__PURE__*/React.createElement(WidgetShell, {
    w: w
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-donut-body"
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-donut"
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-donut-ring",
    style: {
      background: `conic-gradient(${ring})`
    }
  }), /*#__PURE__*/React.createElement("div", {
    className: "ov-donut-hole"
  }, /*#__PURE__*/React.createElement("span", {
    className: "ov-donut-pct"
  }, w.center.pct), /*#__PURE__*/React.createElement("span", {
    className: "ov-donut-cap"
  }, w.center.label))), /*#__PURE__*/React.createElement(RightCol, {
    w: w
  }, /*#__PURE__*/React.createElement(Legend, {
    legend: w.legend,
    active: active,
    setActive: setActive
  }))), /*#__PURE__*/React.createElement(Footer, {
    footer: w.footer
  }));
}
function RightCol({
  w,
  children
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "ov-rightcol"
  }, w.dropdown && /*#__PURE__*/React.createElement(Dropdown, {
    label: w.dropdown
  }), children);
}
function BarWidget({
  w
}) {
  const max = Math.max(...w.bars.map(b => b[1]), 100);
  return /*#__PURE__*/React.createElement(WidgetShell, {
    w: w
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-bar-body"
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-bars"
  }, w.bars.map(([cap, v, color], i) => /*#__PURE__*/React.createElement("div", {
    key: i,
    className: "ov-bar-col"
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-bar-track"
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-bar-fill",
    style: {
      height: `${v / max * 100}%`,
      background: color
    }
  }, cap && /*#__PURE__*/React.createElement("span", {
    className: "ov-bar-cap"
  }, cap)))))), /*#__PURE__*/React.createElement(RightCol, {
    w: w
  }, /*#__PURE__*/React.createElement(Legend, {
    legend: w.legend
  }))), /*#__PURE__*/React.createElement(Footer, {
    footer: w.footer
  }));
}
function TableWidget({
  w
}) {
  return /*#__PURE__*/React.createElement(WidgetShell, {
    w: w
  }, /*#__PURE__*/React.createElement("table", {
    className: "ov-table"
  }, /*#__PURE__*/React.createElement("thead", null, /*#__PURE__*/React.createElement("tr", null, w.cols.map((c, i) => /*#__PURE__*/React.createElement("th", {
    key: i,
    className: i === 0 ? '' : 'rgt'
  }, c, " ", /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sort-alt"
  }))))), /*#__PURE__*/React.createElement("tbody", null, w.rows.map((r, i) => /*#__PURE__*/React.createElement("tr", {
    key: i
  }, /*#__PURE__*/React.createElement("td", null, r[0]), /*#__PURE__*/React.createElement("td", {
    className: "rgt"
  }, r[1]))))), /*#__PURE__*/React.createElement("div", {
    className: "ov-pager"
  }, w.page, " ", /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chevron-left"
  }), " ", /*#__PURE__*/React.createElement("i", {
    className: "pi pi-chevron-right"
  })));
}
function NoDataWidget({
  w
}) {
  return /*#__PURE__*/React.createElement(WidgetShell, {
    w: w
  }, /*#__PURE__*/React.createElement("div", {
    className: "ov-nodata"
  }, "No data available"));
}
function OverviewWidget({
  w
}) {
  if (w.kind === 'donut') return /*#__PURE__*/React.createElement(DonutWidget, {
    w: w
  });
  if (w.kind === 'bar') return /*#__PURE__*/React.createElement(BarWidget, {
    w: w
  });
  if (w.kind === 'table') return /*#__PURE__*/React.createElement(TableWidget, {
    w: w
  });
  return /*#__PURE__*/React.createElement(NoDataWidget, {
    w: w
  });
}
function OverviewControls({
  onRefresh
}) {
  return /*#__PURE__*/React.createElement("div", {
    className: "controls"
  }, /*#__PURE__*/React.createElement("div", {
    className: "controls-left"
  }, /*#__PURE__*/React.createElement("div", {
    className: "daterange"
  }, /*#__PURE__*/React.createElement("span", null, "5/31/26 - 6/6/26"), /*#__PURE__*/React.createElement("span", {
    className: "dr-cal"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-calendar"
  }))), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sort-alt"
  }), " Sort"), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-filter"
  }), " Filters"), /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon",
    title: "Updated: 2 min ago",
    onClick: onRefresh
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-refresh"
  }))), /*#__PURE__*/React.createElement("div", {
    className: "controls-right"
  }, /*#__PURE__*/React.createElement("button", {
    className: "btn btn-outlined btn-icon",
    title: "Settings"
  }, /*#__PURE__*/React.createElement("i", {
    className: "pi pi-sliders-h"
  }))));
}
Object.assign(window, {
  ObservedBar,
  OverviewWidget,
  OverviewControls
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/Widgets.jsx", error: String((e && e.message) || e) }); }

// ui_kits/stratusphere-ux/data.jsx
try { (() => {
/* Shared data for the Stratusphere UX kit — modeled on the real product
   (Overview/Summary, Individual Views, Detailed Views). */

const GFP = {
  good: {
    color: '#16a34a',
    chart: '#22c55e',
    label: 'Good'
  },
  fair: {
    color: '#ca8a04',
    chart: '#eab308',
    label: 'Fair'
  },
  poor: {
    color: '#dc2626',
    chart: '#ef4444',
    label: 'Poor'
  }
};
// chart greens/yellows/reds as used in the product donuts/bars
const C = {
  g: '#3cb44b',
  f: '#f0b400',
  p: '#e02b2b'
};
const DOMAINS = ['CORP.acme.local', 'ENG.acme.local', 'Local Machine'];

// side navigation — mirrors application-side-navigation.ts
const NAV = [{
  items: [{
    key: 'recent',
    label: 'Recent',
    icon: 'pi pi-clock',
    chev: true
  }, {
    key: 'starred',
    label: 'Starred',
    icon: 'pi pi-star',
    chev: true
  }]
}, {
  label: 'Dashboards',
  items: [{
    key: 'overview',
    label: 'Overview',
    icon: 'pi pi-chart-line'
  }, {
    key: 'custom',
    label: 'Custom Dashboards',
    icon: 'pi pi-objects-column'
  }]
}, {
  label: 'Environment',
  items: [{
    key: 'individual',
    label: 'Individual Views',
    mi: 'devices'
  }, {
    key: 'detailed',
    label: 'Detailed Views',
    mi: 'table_view'
  }, {
    key: 'reports',
    label: 'Reports',
    mi: 'summarize'
  }, {
    key: 'legacy-search',
    label: 'Legacy Search',
    mi: 'pageview'
  }, {
    key: 'legacy-spot',
    label: 'Legacy Spot Checks',
    icon: 'pi pi-check-circle'
  }, {
    key: 'legacy-inspector',
    label: 'Legacy Advanced Inspector',
    icon: 'pi pi-table'
  }]
}, {
  label: 'Administration',
  items: [{
    key: 'status',
    label: 'Status',
    icon: 'pi pi-chart-bar'
  }, {
    key: 'configuration',
    label: 'Configuration',
    icon: 'pi pi-cog',
    disabled: true
  }, {
    key: 'inventory',
    label: 'Inventory',
    icon: 'pi pi-receipt'
  }]
}];

// real Overview sub-tabs (overview-tab-list.ts)
const OVERVIEW_TABS = [{
  label: 'Summary',
  icon: 'pi pi-globe'
}, {
  label: 'Resources',
  icon: 'pi pi-database'
}, {
  label: 'Application Experience',
  icon: 'pi pi-microsoft'
}, {
  label: 'Login Experience',
  icon: 'pi pi-user'
}, {
  label: 'Asset Management',
  icon: 'pi pi-desktop'
}];

// Individual-view tabs (en.json overviewTabs)
const IV_TABS = ['Summary', 'CPU', 'RAM', 'Disk', 'Network', 'Applications & Processes', 'Login & Logoff', 'Remote Session Display', 'Events & Alerts', 'Browser', 'Trending'];

// the two "observed" summary bars
const OBSERVED = [{
  id: 'machines',
  label: 'Machines Observed',
  icon: 'pi pi-desktop',
  amount: 16,
  total: 27
}, {
  id: 'users',
  label: 'Users Observed',
  icon: 'pi pi-users',
  amount: 5,
  total: 95
}];

// Overview/Summary widgets — kind: donut | bar | table | nodata
const OV_WIDGETS = [{
  id: 'mux',
  title: 'Machines by UX',
  icon: 'pi pi-desktop',
  link: true,
  kind: 'donut',
  center: {
    pct: '0%',
    label: 'Poor UX'
  },
  legend: [['Good UX', 11, C.g], ['Fair UX', 5, C.f], ['Poor UX', 0, C.p]],
  footer: [['Min', '2.19', C.f], ['Avg', '2.81', C.f], ['Peak Avg', '2.83', C.g]]
}, {
  id: 'uux',
  title: 'Users by UX',
  icon: 'pi pi-users',
  link: true,
  kind: 'donut',
  center: {
    pct: '0%',
    label: 'Poor UX'
  },
  legend: [['Good UX', 4, C.g], ['Fair UX', 1, C.f], ['Poor UX', 0, C.p]],
  footer: [['Min', '2.19', C.f], ['Avg', '2.75', C.f], ['Peak Avg', '2.8', C.g]]
}, {
  id: 'nsr',
  title: 'Machines by Network Security Risk',
  icon: 'pi pi-desktop',
  link: true,
  kind: 'bar',
  dropdown: 'VMware',
  bars: [['', 8, C.g], ['', 0, C.f], ['', 91, C.p]],
  legend: [['Low', 1, C.g], ['Medium', 0, C.f], ['High', 11, C.p]],
  footer: [['Avg', 'High', C.p], ['Peak', 'High', C.p]]
}, {
  id: 'wi',
  title: 'Workspace Impact',
  mi: 'work',
  link: true,
  kind: 'bar',
  bars: [['58%', 58, C.g], ['35%', 35, C.f], ['5%', 5, C.p]],
  legend: [['Low', 10, C.g], ['Medium', 6, C.f], ['High', 1, C.p]],
  footer: [['Avg', 'Low', C.g], ['Peak Avg', 'High', C.p], ['Peak', 'High', C.p]]
}, {
  id: 'mtux',
  title: 'Machine Types by UX',
  icon: 'pi pi-desktop',
  kind: 'bar',
  dropdown: 'VMware',
  bars: [['66%', 66, C.g], ['33%', 33, C.f], ['0%', 0, C.p]],
  legend: [['Good UX', 8, C.g], ['Fair UX', 4, C.f], ['Poor UX', 0, C.p]],
  footer: [['Min', '2.62', C.f], ['Avg', '2.81', C.f], ['Peak Avg', '2.84', C.g]]
}, {
  id: 'os',
  title: 'Operating Systems',
  icon: 'pi pi-microsoft',
  kind: 'donut',
  dropdown: 'Oracle Linux Server',
  center: {
    pct: '0%',
    label: 'Poor'
  },
  legend: [['Good', 4, C.g], ['Fair', 4, C.f], ['Poor', 0, C.p]],
  footer: [['Min', '2.62', C.f], ['Avg', '2.77', C.f], ['Peak Avg', '2.8', C.g]]
}, {
  id: 'upt',
  title: 'Machine Uptimes by Days',
  icon: 'pi pi-desktop',
  kind: 'donut',
  center: {
    pct: '25%',
    label: '> 28'
  },
  mix: [9, 3, 4],
  legend: [['<= 14', 9, C.g], ['14 - 28', 3, C.f], ['> 28', 4, C.p]],
  footer: [['Avg', '38.8 days', C.p], ['Peak Avg', '40.1 days', C.p], ['Peak', '212.4 days', C.p]]
}, {
  id: 'ldu',
  title: 'Login Delay UX by User',
  icon: 'pi pi-users',
  kind: 'donut',
  center: {
    pct: '0%',
    label: '> 21s'
  },
  mix: [1, 0, 0],
  legend: [['<= 12s', 1, C.g], ['12 - 21s', 0, C.f], ['> 21s', 0, C.p]],
  footer: [['Avg', '0.3 s', C.g], ['Peak Avg', '1 s', C.g], ['Peak', '1 s', C.g]]
}, {
  id: 'tuwr',
  title: 'Top 10 Users by Workload Ranking',
  icon: 'pi pi-users',
  kind: 'table',
  cols: ['USER', 'RANK'],
  rows: [['nalba', 1], ['friend', 2], ['DnyaneshKhare', 3], ['lwl', 4], ['thinos', 5]],
  page: '1 of 1'
}, {
  id: 'tmwr',
  title: 'Top 10 Machines by Workload Ranking',
  icon: 'pi pi-desktop',
  kind: 'table',
  cols: ['MACHINE', 'RANK'],
  rows: [['nalba-latitude', 1], ['vDKhare-WinXl-01', 2], ['ubuntu2004build-1.trustednetwork.biz', 3], ['devhub.trustednetwork.biz', 4], ['centos8build.trustednetwork.biz', 5]],
  page: '1 of 2'
}, {
  id: 'ldt',
  title: 'Login Delay UX by Type',
  icon: 'pi pi-users',
  kind: 'bar',
  bars: [['100%', 100, C.g]],
  legend: [['<= 12s', 1, C.g], ['12 - 21s', 0, C.f], ['> 21s', 0, C.p]],
  footer: [['Avg', '0.3 s', C.g], ['Peak Avg', '1 s', C.g], ['Peak', '1 s', C.g]]
}, {
  id: 'rslc',
  title: 'Remote Session Latency Counts',
  icon: 'pi pi-wifi',
  kind: 'nodata'
}];

// Individual Views — detail cards for one machine/user
const IV_CARDS = [{
  id: 'machine',
  title: 'Machine',
  icon: 'pi pi-desktop',
  big: 'desktop-c65gr6p',
  rows: [['Type', 'Physical'], ['Make', 'Dell Inc.'], ['Model', 'Latitude 5530'], ['Location', 'Alpharetta, Georgia US']]
}, {
  id: 'os',
  title: 'Operating System',
  icon: 'pi pi-microsoft',
  big: 'Windows 11',
  rows: [['Edition', 'Windows 11 Pro'], ['Version', '25H2'], ['Boot Time', 'Nov 13 2025, 10:55 am']]
}, {
  id: 'ux',
  title: 'User Experience',
  mi: 'insights',
  big: '81.6%',
  sub: 'UX Score: 2.45',
  chart: 'area-flat',
  link: false,
  rows: [['App Launch', '19.8s'], ['ANR Count', '0']]
}, {
  id: 'user',
  title: 'User Information',
  icon: 'pi pi-user',
  big: 'DnyaneshKhare',
  rows: [['UX Rating', 'Fair'], ['Last Seen', 'Nov 18 2025, 09:54 am'], ['Admin', 'No']]
}, {
  id: 'cpu',
  title: 'CPU Utilization',
  icon: 'pi pi-chart-line',
  link: true,
  big: '60%',
  sub: '12th Gen Intel® Core™ i7-1265U',
  chart: 'area-spiky',
  rows: [['Allocated', '18000 MHz'], ['Peak Avg', '16558.2 MHz, 92%']]
}, {
  id: 'ram',
  title: 'RAM Utilization',
  icon: 'pi pi-server',
  link: true,
  big: '50%',
  chart: 'area-rise',
  rows: [['Allocated', '63.69 GB'], ['Peak Avg', '32.1 GB, 50.4%']]
}, {
  id: 'disk',
  title: 'Disk Utilization',
  mi: 'disc_full',
  link: true,
  big: '99%',
  chart: 'bar-block',
  rows: [['Avg Latency', '0.07 ms']]
}, {
  id: 'net',
  title: 'Network Utilization',
  icon: 'pi pi-wifi',
  link: true,
  big: '310.5 KB/s',
  sub: 'Intel® Wi-Fi 6E AX211 160MHz, Realtek USB GbE Family Controller',
  chart: 'area-peak',
  rows: [['Link Speed', '912 MBit/s'], ['Peak Avg', '13767 KB/s']]
}, {
  id: 'gpu',
  title: 'GPU Utilization',
  icon: 'pi pi-desktop',
  empty: true
}, {
  id: 'npu',
  title: 'NPU Utilization',
  icon: 'pi pi-server',
  empty: true
}, {
  id: 'procload',
  title: 'Top Processes by Load Time',
  mi: 'format_list_bulleted',
  stub: 'table'
}, {
  id: 'procfg',
  title: 'Top Processes by User Foreground Focus',
  mi: 'format_list_bulleted',
  stub: 'table'
}];

// Detailed Views — heatmap data table (real columns + values + s-colors from tmp-data.ts)
const DV_COLS = [{
  k: 'user',
  label: 'User',
  align: 'r'
}, {
  k: 'ux',
  label: 'UX Score',
  align: 'c',
  grade: true
}, {
  k: 'login',
  label: 'Login Delay',
  align: 'r'
}, {
  k: 'load',
  label: 'App Load Time',
  align: 'r'
}, {
  k: 'anr',
  label: 'App Not Resp (ANR)',
  align: 'c'
}, {
  k: 'cpu',
  label: 'CPU Used %',
  align: 'r'
}, {
  k: 'queue',
  label: 'CPU Queue',
  align: 'r'
}, {
  k: 'ctx',
  label: 'Context Switching',
  align: 'r'
}, {
  k: 'mem',
  label: 'Memory Used %',
  align: 'r'
}, {
  k: 'page',
  label: 'Pagefile Used %',
  align: 'r'
}, {
  k: 'soft',
  label: 'Soft Page Faults',
  align: 'r'
}, {
  k: 'hard',
  label: 'Hard Page Faults',
  align: 'r'
}, {
  k: 'gdi',
  label: 'GDI Objects',
  align: 'r'
}, {
  k: 'gpu',
  label: 'GPU Core Used %',
  align: 'r'
}, {
  k: 'gmem',
  label: 'GPU Memory Used MB',
  align: 'r'
}];
// cell = value, or [value, bgColor, textColor]
const Y = '#FFFF99',
  O = '#FFCC66',
  R = '#ff6666';
const grade = (g, c) => [g, c, '#000'];
const DV_ROWS = [{
  user: 'zgu',
  ux: grade('B+', Y),
  login: 0,
  load: 1.4,
  anr: 0,
  cpu: 17.1,
  queue: ['11.17', R, '#fff'],
  ctx: 126175,
  mem: 77.8,
  page: 7.7,
  soft: ['20093', R, '#fff'],
  hard: ['3491', R, '#fff'],
  gdi: 82,
  gpu: 0.0,
  gmem: 1018
}, {
  user: 'nalba',
  ux: grade('B+', Y),
  login: 1,
  load: ['3.9', Y],
  anr: 0,
  cpu: 12.6,
  queue: ['1.68', O],
  ctx: 15504,
  mem: ['83.4', Y],
  page: 9.1,
  soft: ['7534', Y],
  hard: ['1511', R, '#fff'],
  gdi: 56,
  gpu: 2.9,
  gmem: 1289
}, {
  user: 'dnyaneshkhare',
  ux: grade('A-', '#01DF01'),
  login: 0,
  load: 0.0,
  anr: 0,
  cpu: 10.6,
  queue: ['1.52', O],
  ctx: 2822,
  mem: 66.4,
  page: ['12.4', Y],
  soft: 1214,
  hard: 74,
  gdi: 40,
  gpu: 0.0,
  gmem: 0
}, {
  user: 'NO_LOGIN_USER',
  ux: grade('A-', '#01DF01'),
  login: 0,
  load: 0.0,
  anr: 0,
  cpu: 5.0,
  queue: ['1.49', O],
  ctx: 823,
  mem: 46.0,
  page: ['19.5', Y],
  soft: 848,
  hard: 21,
  gdi: 0,
  gpu: 0.0,
  gmem: 0
}, {
  user: 'friend',
  ux: grade('A', '#15c015'),
  login: 0,
  load: ['2.2', Y],
  anr: 0,
  cpu: 8.7,
  queue: ['1.55', O],
  ctx: 435,
  mem: 48.4,
  page: 8.6,
  soft: 471,
  hard: 13,
  gdi: 0,
  gpu: ['38.3', Y],
  gmem: 252
}, {
  user: 'thinos',
  ux: grade('A+', '#00d000'),
  login: 0,
  load: 0.0,
  anr: 0,
  cpu: 0.6,
  queue: ['1.06', O],
  ctx: 726,
  mem: 17.2,
  page: 0.0,
  soft: 0,
  hard: 0,
  gdi: 0,
  gpu: 0.0,
  gmem: 0
}];
const fmt = n => typeof n === 'number' ? n.toLocaleString('en-US') : n;
Object.assign(window, {
  GFP,
  C,
  DOMAINS,
  NAV,
  OVERVIEW_TABS,
  IV_TABS,
  OBSERVED,
  OV_WIDGETS,
  IV_CARDS,
  DV_COLS,
  DV_ROWS,
  fmt
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/stratusphere-ux/data.jsx", error: String((e && e.message) || e) }); }

})();
