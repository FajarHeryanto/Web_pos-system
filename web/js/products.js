(function () {
  'use strict';
  const body = document.getElementById('products-body');
  const count = document.getElementById('product-count');
  const categoryFilter = document.getElementById('category-filter');
  const search = document.getElementById('product-search');
  const empty = document.getElementById('product-empty');
  const form = document.getElementById('product-form');
  const modal = new bootstrap.Modal(document.getElementById('product-modal'));
  const deleteModal = new bootstrap.Modal(document.getElementById('delete-modal'));
  let products = [];
  let editingID = '';
  let deletingID = '';
  let debounceTimer;
  const initialSearch = new URLSearchParams(location.search).get('search');
  if (initialSearch) search.value = initialSearch;

  const addButton = document.getElementById('add-product');
  if (!POSIsAdmin) addButton.classList.add('d-none');

  function iconFor(category) {
    const value = String(category || '').toLowerCase();
    if (value.includes('minum')) return 'bi-cup-straw';
    if (value.includes('makan')) return 'bi-egg-fried';
    if (value.includes('elektr')) return 'bi-headphones';
    if (value.includes('tulis')) return 'bi-pencil-square';
    if (value.includes('rt')) return 'bi-house-heart';
    return 'bi-box-seam';
  }
  function updateCategories(rows) {
    const current = categoryFilter.value;
    const categories = Array.from(new Set(rows.map(function (p) { return p.category; }))).sort();
    categoryFilter.innerHTML = '<option value="">Semua kategori</option>' + categories.map(function (value) { return '<option value="' + POSApi.escape(value) + '">' + POSApi.escape(value) + '</option>'; }).join('');
    if (categories.includes(current)) categoryFilter.value = current;
    document.getElementById('category-options').innerHTML = categories.map(function (value) { return '<option value="' + POSApi.escape(value) + '"></option>'; }).join('');
  }
  function render(rows) {
    count.textContent = rows.length.toLocaleString('id-ID') + ' produk';
    if (!rows.length) {
      body.innerHTML = '';
      empty.classList.remove('d-none');
      return;
    }
    empty.classList.add('d-none');
    body.innerHTML = rows.map(function (p) {
      let status = '<span class="stock-pill stock-good"><i class="bi bi-check-circle-fill"></i>Stok tersedia</span>';
      if (p.stock <= 0) status = '<span class="stock-pill stock-out"><i class="bi bi-x-circle-fill"></i>Habis</span>';
      else if (p.stock <= 10) status = '<span class="stock-pill stock-low"><i class="bi bi-exclamation-circle-fill"></i>Stok menipis</span>';
      const actions = POSIsAdmin ? '<div class="table-actions"><button class="icon-btn edit-product" type="button" title="Ubah produk" aria-label="Ubah ' + POSApi.escape(p.name) + '" data-id="' + POSApi.escape(p.product_id) + '"><i class="bi bi-pencil"></i></button><button class="icon-btn danger delete-product" type="button" title="Hapus produk" aria-label="Hapus ' + POSApi.escape(p.name) + '" data-id="' + POSApi.escape(p.product_id) + '"><i class="bi bi-trash3"></i></button></div>' : '<span class="text-muted">—</span>';
      return '<tr><td><div class="product-name-cell"><span class="product-avatar"><i class="bi ' + iconFor(p.category) + '"></i></span><span><strong>' + POSApi.escape(p.name) + '</strong><small class="d-block">' + POSApi.escape(p.product_id) + '</small></span></div></td><td class="category-cell">' + POSApi.escape(p.category) + '</td><td class="product-price">' + POSApi.money(p.price) + '</td><td><strong>' + Number(p.stock).toLocaleString('id-ID') + '</strong></td><td>' + status + '</td><td class="text-end">' + actions + '</td></tr>';
    }).join('');
  }
  async function load() {
    body.innerHTML = '<tr><td colspan="6" class="text-center text-muted py-5">Memuat produk…</td></tr>';
    try {
      const params = new URLSearchParams();
      if (search.value.trim()) params.set('search', search.value.trim());
      if (categoryFilter.value) params.set('category', categoryFilter.value);
      const rows = await POSApi.request('/products' + (params.size ? '?' + params.toString() : ''));
      products = rows || [];
      if (!search.value.trim() && !categoryFilter.value) updateCategories(products);
      render(products);
    } catch (error) {
      body.innerHTML = '<tr><td colspan="6" class="text-center text-danger py-5">' + POSApi.escape(POSApi.formatError(error)) + '</td></tr>';
      count.textContent = '—';
    }
  }
  function openCreate() {
    editingID = '';
    form.reset();
    document.getElementById('product-modal-title').textContent = 'Tambah produk';
    document.getElementById('form-product-id').disabled = false;
    document.getElementById('form-product-id').value = '';
    document.getElementById('product-form-error').innerHTML = '';
    document.querySelector('#save-product .save-label').textContent = 'Simpan produk';
    modal.show();
  }
  function openEdit(product) {
    editingID = product.product_id;
    document.getElementById('product-modal-title').textContent = 'Ubah produk';
    document.getElementById('form-product-id').value = product.product_id;
    document.getElementById('form-product-id').disabled = true;
    document.getElementById('form-product-name').value = product.name;
    document.getElementById('form-product-category').value = product.category;
    document.getElementById('form-product-price').value = product.price;
    document.getElementById('form-product-stock').value = product.stock;
    document.getElementById('product-form-error').innerHTML = '';
    document.querySelector('#save-product .save-label').textContent = 'Simpan perubahan';
    modal.show();
  }
  addButton.addEventListener('click', openCreate);
  categoryFilter.addEventListener('change', load);
  search.addEventListener('input', function () { clearTimeout(debounceTimer); debounceTimer = setTimeout(load, 250); });
  body.addEventListener('click', function (event) {
    const edit = event.target.closest('.edit-product');
    if (edit) {
      const item = products.find(function (p) { return p.product_id === edit.dataset.id; });
      if (item) openEdit(item);
    }
    const remove = event.target.closest('.delete-product');
    if (remove) {
      const item = products.find(function (p) { return p.product_id === remove.dataset.id; });
      if (!item) return;
      deletingID = item.product_id;
      document.getElementById('delete-product-name').textContent = '“' + item.name + '” akan dihapus dari katalog. Produk yang memiliki histori transaksi tidak dapat dihapus.';
      deleteModal.show();
    }
  });
  form.addEventListener('submit', async function (event) {
    event.preventDefault();
    const save = document.getElementById('save-product');
    save.disabled = true;
    save.querySelector('.spinner-border').classList.remove('d-none');
    const payload = {
      product_id: document.getElementById('form-product-id').value.trim(),
      name: document.getElementById('form-product-name').value.trim(),
      category: document.getElementById('form-product-category').value.trim(),
      price: Number(document.getElementById('form-product-price').value),
      stock: Number(document.getElementById('form-product-stock').value)
    };
    try {
      await POSApi.request(editingID ? '/products/' + encodeURIComponent(editingID) : '/products', {
        method: editingID ? 'PUT' : 'POST', body: payload
      });
      modal.hide();
      POSApi.showAlert('#product-alert', editingID ? 'Produk berhasil diperbarui.' : 'Produk berhasil ditambahkan.', 'success');
      await load();
    } catch (error) {
      POSApi.showAlert('#product-form-error', POSApi.formatError(error));
    } finally {
      save.disabled = false;
      save.querySelector('.spinner-border').classList.add('d-none');
    }
  });
  document.getElementById('confirm-delete').addEventListener('click', async function () {
    const button = this;
    button.disabled = true;
    try {
      await POSApi.request('/products/' + encodeURIComponent(deletingID), { method: 'DELETE' });
      deleteModal.hide();
      POSApi.showAlert('#product-alert', 'Produk berhasil dihapus.', 'success');
      await load();
    } catch (error) {
      deleteModal.hide();
      POSApi.showAlert('#product-alert', POSApi.formatError(error));
    } finally { button.disabled = false; }
  });
  load();
})();
