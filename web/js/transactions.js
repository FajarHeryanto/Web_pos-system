(function () {
  'use strict';
  let products = [];
  const cart = new Map();
  let lastReceipt = null;
  let activeCategory = '';
  let debounceTimer;
  const productHost = document.getElementById('pos-products');
  const categoryHost = document.getElementById('category-pills');
  const cartHost = document.getElementById('cart-items');
  const checkout = document.getElementById('checkout-btn');
  const historyPanel = document.getElementById('history-panel');
  const receiptModal = new bootstrap.Modal(document.getElementById('receipt-modal'));

  function iconFor(category) {
    const value = String(category || '').toLowerCase();
    if (value.includes('minum')) return 'bi-cup-straw';
    if (value.includes('makan')) return 'bi-egg-fried';
    if (value.includes('elektr')) return 'bi-headphones';
    if (value.includes('tulis')) return 'bi-pencil-square';
    if (value.includes('rt')) return 'bi-house-heart';
    return 'bi-box-seam';
  }
  function renderCategories() {
    const categories = Array.from(new Set(products.map(function (p) { return p.category; }))).sort();
    const all = [''].concat(categories);
    categoryHost.innerHTML = all.map(function (category) {
      const label = category || 'Semua produk';
      return '<button class="category-pill ' + (activeCategory === category ? 'active' : '') + '" type="button" data-category="' + POSApi.escape(category) + '">' + POSApi.escape(label) + '</button>';
    }).join('');
  }
  function filteredProducts() {
    const q = document.getElementById('pos-search').value.trim().toLowerCase();
    return products.filter(function (p) {
      return (!activeCategory || p.category === activeCategory) && (!q || p.name.toLowerCase().includes(q) || p.product_id.toLowerCase().includes(q));
    });
  }
  function renderProducts() {
    const rows = filteredProducts();
    document.getElementById('pos-product-count').textContent = rows.length + ' produk';
    if (!rows.length) {
      productHost.innerHTML = '<div class="pos-loading">Produk tidak ditemukan.</div>';
      return;
    }
    productHost.innerHTML = rows.map(function (p) {
      const disabled = p.stock <= 0;
      return '<article class="pos-product ' + (disabled ? 'product-disabled' : '') + '" role="button" tabindex="' + (disabled ? '-1' : '0') + '" aria-label="' + POSApi.escape(p.name + ', ' + POSApi.money(p.price) + ', stok ' + p.stock) + '" data-id="' + POSApi.escape(p.product_id) + '" aria-disabled="' + disabled + '"><span class="pos-product-icon"><i class="bi ' + iconFor(p.category) + '"></i></span><strong>' + POSApi.escape(p.name) + '</strong><span class="pos-category">' + POSApi.escape(p.category) + '</span><span class="pos-price">' + POSApi.money(p.price) + '</span><span class="pos-stock">Stok ' + Number(p.stock).toLocaleString('id-ID') + '</span></article>';
    }).join('');
  }
  function cartTotals() {
    let subtotal = 0;
    cart.forEach(function (item) { subtotal += item.price * item.quantity; });
    subtotal = Math.round(subtotal * 100) / 100;
    const percent = Math.max(0, Math.min(100, Number(document.getElementById('discount-input').value) || 0));
    const discount = Math.round(subtotal * percent) / 100;
    return { subtotal: subtotal, discount: discount, total: Math.round((subtotal - discount) * 100) / 100, percent: percent };
  }
  function renderCart() {
    if (!cart.size) {
      cartHost.innerHTML = '<div class="cart-empty"><i class="bi bi-basket2"></i><p>Keranjang masih kosong</p><span>Pilih produk untuk memulai.</span></div>';
    } else {
      cartHost.innerHTML = Array.from(cart.values()).map(function (item) {
        return '<div class="cart-row"><div><div class="cart-row-name" title="' + POSApi.escape(item.name) + '">' + POSApi.escape(item.name) + '</div><div class="cart-row-price">' + POSApi.money(item.price) + ' / unit</div></div><div class="qty-control"><button type="button" class="qty-minus" data-id="' + POSApi.escape(item.id) + '" aria-label="Kurangi ' + POSApi.escape(item.name) + '"><i class="bi bi-dash"></i></button><span>' + item.quantity + '</span><button type="button" class="qty-plus" data-id="' + POSApi.escape(item.id) + '" aria-label="Tambah ' + POSApi.escape(item.name) + '" ' + (item.quantity >= item.stock ? 'disabled' : '') + '><i class="bi bi-plus"></i></button></div></div>';
      }).join('');
    }
    const totals = cartTotals();
    document.getElementById('cart-count').textContent = cart.size + (cart.size === 1 ? ' produk dipilih' : ' produk dipilih');
    document.getElementById('cart-subtotal').textContent = POSApi.money(totals.subtotal);
    document.getElementById('cart-discount').textContent = '− ' + POSApi.money(totals.discount);
    document.getElementById('cart-total').textContent = POSApi.money(totals.total);
    checkout.disabled = !cart.size;
    document.getElementById('clear-cart').disabled = !cart.size;
    document.getElementById('print-receipt').disabled = !lastReceipt;
  }
  function addProduct(id) {
    const product = products.find(function (p) { return p.product_id === id; });
    if (!product || product.stock <= 0) return;
    const existing = cart.get(id);
    if (existing && existing.quantity >= product.stock) {
      POSApi.showAlert('#transaction-alert', 'Jumlah ' + product.name + ' sudah mencapai batas stok.');
      return;
    }
    cart.set(id, { id: id, name: product.name, price: Number(product.price), quantity: existing ? existing.quantity + 1 : 1, stock: Number(product.stock) });
    renderCart();
  }
  function alterQuantity(id, delta) {
    const item = cart.get(id);
    if (!item) return;
    const next = item.quantity + delta;
    if (next <= 0) cart.delete(id);
    else if (next <= item.stock) item.quantity = next;
    renderCart();
  }
  async function loadProducts() {
    try {
      products = await POSApi.request('/products') || [];
      renderCategories();
      renderProducts();
    } catch (error) {
      productHost.innerHTML = '<div class="pos-loading text-danger">' + POSApi.escape(POSApi.formatError(error)) + '</div>';
    }
  }
  categoryHost.addEventListener('click', function (event) {
    const button = event.target.closest('[data-category]');
    if (!button) return;
    activeCategory = button.dataset.category;
    renderCategories();
    renderProducts();
  });
  productHost.addEventListener('click', function (event) {
    const card = event.target.closest('.pos-product');
    if (card && card.getAttribute('aria-disabled') !== 'true') addProduct(card.dataset.id);
  });
  productHost.addEventListener('keydown', function (event) {
    if ((event.key === 'Enter' || event.key === ' ') && event.target.matches('.pos-product')) {
      event.preventDefault();
      if (event.target.getAttribute('aria-disabled') !== 'true') addProduct(event.target.dataset.id);
    }
  });
  cartHost.addEventListener('click', function (event) {
    const minus = event.target.closest('.qty-minus');
    const plus = event.target.closest('.qty-plus');
    if (minus) alterQuantity(minus.dataset.id, -1);
    if (plus) alterQuantity(plus.dataset.id, 1);
  });
  document.getElementById('pos-search').addEventListener('input', function () { clearTimeout(debounceTimer); debounceTimer = setTimeout(renderProducts, 120); });
  document.getElementById('discount-input').addEventListener('input', renderCart);
  document.getElementById('clear-cart').addEventListener('click', function () { cart.clear(); renderCart(); });

  function renderReceipt(data) {
    const items = data.transactions || [];
    const total = Number(data.total || 0);
    const date = items.length ? items[0].transaction_date : new Date().toISOString().slice(0, 10);
    const itemHTML = items.map(function (item) {
      return '<div class="receipt-line"><span>' + POSApi.escape(item.product_name) + ' × ' + Number(item.quantity) + (Number(item.discount_percent) ? ' (' + Number(item.discount_percent) + '%)' : '') + '</span><strong>' + POSApi.money(item.total_price) + '</strong></div>';
    }).join('');
    document.getElementById('receipt-body').innerHTML = '<div class="receipt-store"><strong>POS TOKO</strong><span>Struk pembayaran</span></div><div class="receipt-meta"><span>No. ' + POSApi.escape(items.map(function (i) { return i.transaction_id; }).join(', ')) + '</span><span class="text-end">' + POSApi.date(date) + '</span><span>Kasir: ' + POSApi.escape(items[0] ? items[0].cashier_name : '') + '</span><span class="text-end">' + POSApi.escape(items[0] ? items[0].payment_method : '') + '</span></div><div class="receipt-lines">' + itemHTML + '</div><div class="receipt-total"><span>Total pembayaran</span><strong>' + POSApi.money(total) + '</strong></div><p class="text-center text-muted mt-3 mb-0" style="font-size:9px">Terima kasih atas kunjungan Anda.</p>';
  }
  checkout.addEventListener('click', async function () {
    if (!cart.size) return;
    checkout.disabled = true;
    checkout.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>Menyimpan…';
    const discount = cartTotals().percent;
    const payload = {
      items: Array.from(cart.values()).map(function (item) { return { product_id: item.id, quantity: item.quantity, discount_percent: discount }; }),
      payment_method: document.getElementById('payment-method').value,
      transaction_date: new Date().toISOString().slice(0, 10)
    };
    try {
      const data = await POSApi.request('/transactions', { method: 'POST', body: payload });
      lastReceipt = data;
      cart.clear();
      document.getElementById('discount-input').value = 0;
      renderCart();
      renderReceipt(data);
      receiptModal.show();
      POSApi.showAlert('#transaction-alert', 'Transaksi berhasil disimpan dan stok telah diperbarui.', 'success');
      await loadProducts();
      if (!historyPanel.classList.contains('d-none')) await loadHistory();
    } catch (error) {
      POSApi.showAlert('#transaction-alert', POSApi.formatError(error));
    } finally {
      checkout.disabled = cart.size === 0;
      checkout.innerHTML = '<i class="bi bi-check2-circle me-2"></i>Simpan transaksi';
    }
  });
  async function loadHistory() {
    const historyBody = document.getElementById('history-body');
    historyBody.innerHTML = '<tr><td colspan="6" class="text-center text-muted py-4">Memuat transaksi…</td></tr>';
    try {
      const rows = await POSApi.request('/transactions?limit=100');
      if (!rows.length) { historyBody.innerHTML = '<tr><td colspan="6" class="text-center text-muted py-4">Belum ada transaksi.</td></tr>'; return; }
      historyBody.innerHTML = rows.map(function (item) {
        return '<tr><td class="table-id">' + POSApi.escape(item.transaction_id) + '</td><td class="table-primary-label">' + POSApi.escape(item.product_name) + ' <span class="text-muted">× ' + item.quantity + '</span></td><td>' + POSApi.escape(item.cashier_name) + '</td><td>' + POSApi.date(item.transaction_date) + '</td><td><span class="payment-pill">' + POSApi.escape(item.payment_method) + '</span></td><td class="text-end table-total">' + POSApi.money(item.total_price) + '</td></tr>';
      }).join('');
    } catch (error) { historyBody.innerHTML = '<tr><td colspan="6" class="text-center text-danger py-4">' + POSApi.escape(POSApi.formatError(error)) + '</td></tr>'; }
  }
  document.getElementById('show-history').addEventListener('click', async function () { historyPanel.classList.remove('d-none'); await loadHistory(); historyPanel.scrollIntoView({ behavior: 'smooth', block: 'start' }); });
  document.getElementById('hide-history').addEventListener('click', function () { historyPanel.classList.add('d-none'); });
  function printReceipt() { document.body.classList.add('print-receipt'); window.print(); setTimeout(function () { document.body.classList.remove('print-receipt'); }, 500); }
  document.getElementById('print-receipt').addEventListener('click', function () { if (!lastReceipt) return; renderReceipt(lastReceipt); receiptModal.show(); setTimeout(printReceipt, 250); });
  document.getElementById('print-modal-receipt').addEventListener('click', printReceipt);
  renderCart();
  loadProducts();
})();
