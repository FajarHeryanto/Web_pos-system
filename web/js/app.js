(function () {
  'use strict';
  const user = POSApi.getUser();
  if (!POSApi.getToken()) { POSApi.redirectLogin(); return; }
  const page = document.body.dataset.page || 'dashboard';
  const nav = [
    { id: 'dashboard', label: 'Dashboard', href: '/', icon: 'bi-grid-1x2' },
    { id: 'products', label: 'Produk', href: '/products', icon: 'bi-box-seam' },
    { id: 'transactions', label: 'Transaksi', href: '/transactions', icon: 'bi-receipt' },
    { id: 'reports', label: 'Laporan', href: '/reports', icon: 'bi-bar-chart-line' }
  ];
  const isAdmin = user && user.role === 'admin';
  const sidebarMount = document.getElementById('sidebar-mount');
  if (sidebarMount) {
    sidebarMount.innerHTML = '<aside class="sidebar" id="app-sidebar"><a href="/" class="brand"><span class="brand-mark"><i class="bi bi-bag-check-fill"></i></span><span>POS Toko</span></a><p class="store-caption">RUANG KERJA TOKO</p><nav class="sidebar-nav" aria-label="Navigasi utama">' + nav.map(function (item) {
      return '<a class="nav-item ' + (item.id === page ? 'active' : '') + '" href="' + item.href + '"' + (item.id === page ? ' aria-current="page"' : '') + '><i class="bi ' + item.icon + '"></i><span>' + item.label + '</span></a>';
    }).join('') + '</nav><div class="sidebar-bottom"><div class="sidebar-help"><span class="help-mark"><i class="bi bi-headset"></i></span><strong>Butuh bantuan?</strong><p>Hubungi administrator toko Anda.</p></div><button class="logout-btn" id="logout-btn" type="button"><i class="bi bi-box-arrow-left"></i><span>Keluar</span></button></div></aside><div class="sidebar-backdrop" id="sidebar-backdrop"></div>';
  }
  const topbarMount = document.getElementById('topbar-mount');
  if (topbarMount) {
    const current = nav.find(function (item) { return item.id === page; });
    topbarMount.innerHTML = '<header class="topbar"><div class="topbar-left"><button class="mobile-menu" id="mobile-menu" type="button" aria-label="Buka navigasi"><i class="bi bi-list"></i></button><span class="topbar-context">' + (current ? current.label : 'POS Toko') + '</span></div><div class="topbar-right"><label class="topbar-search"><i class="bi bi-search"></i><input id="global-search" type="search" class="form-control" placeholder="Cari produk atau transaksi…" aria-label="Pencarian cepat"></label><span class="topbar-divider"></span><div class="user-chip"><span class="user-avatar">' + POSApi.escape(POSApi.initials(user && user.full_name)) + '</span><span class="user-info"><strong>' + POSApi.escape(user && user.full_name || 'Pengguna') + '</strong><span>' + POSApi.escape(user && user.role || 'Kasir') + '</span></span></div></div></header>';
  }
  document.addEventListener('click', function (event) {
    if (event.target.closest('#logout-btn')) {
      POSApi.clearSession();
      location.replace('/login');
    }
    if (event.target.closest('#mobile-menu')) {
      const sidebar = document.getElementById('app-sidebar');
      const backdrop = document.getElementById('sidebar-backdrop');
      sidebar.classList.toggle('open');
      backdrop.classList.toggle('show');
    }
    if (event.target.closest('#sidebar-backdrop')) {
      document.getElementById('app-sidebar').classList.remove('open');
      event.target.classList.remove('show');
    }
  });
  const globalSearch = document.getElementById('global-search');
  if (globalSearch) globalSearch.addEventListener('input', function () {
    const target = page === 'products' ? document.getElementById('product-search') :
      page === 'transactions' ? document.getElementById('pos-search') : null;
    if (target) { target.value = globalSearch.value; target.dispatchEvent(new Event('input')); }
  });
  if (globalSearch && !['products', 'transactions'].includes(page)) {
    globalSearch.addEventListener('keydown', function (event) {
      if (event.key === 'Enter' && globalSearch.value.trim()) {
        location.href = '/products?search=' + encodeURIComponent(globalSearch.value.trim());
      }
    });
  }
  window.POSUser = user;
  window.POSIsAdmin = !!isAdmin;
  document.documentElement.dataset.role = isAdmin ? 'admin' : 'kasir';
})();
