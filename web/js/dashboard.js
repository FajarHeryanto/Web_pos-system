(function () {
  'use strict';
  const today = document.getElementById('today-label');
  today.textContent = new Intl.DateTimeFormat('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }).format(new Date());
  const alertBox = document.getElementById('dashboard-alert');

  function renderRecent(rows) {
    const body = document.getElementById('recent-transactions');
    if (!rows.length) {
      body.innerHTML = '<tr><td colspan="6" class="text-center text-muted py-4">Belum ada transaksi. Mulai penjualan melalui menu Transaksi.</td></tr>';
      return;
    }
    body.innerHTML = rows.map(function (item) {
      return '<tr><td class="table-id">' + POSApi.escape(item.transaction_id) + '</td><td class="table-primary-label">' + POSApi.escape(item.product_name) + '</td><td>' + POSApi.escape(item.cashier_name) + '</td><td>' + POSApi.date(item.transaction_date) + '</td><td><span class="payment-pill">' + POSApi.escape(item.payment_method) + '</span></td><td class="text-end table-total">' + POSApi.money(item.total_price) + '</td></tr>';
    }).join('');
  }
  function renderTop(rows) {
    const host = document.getElementById('top-products');
    if (!rows.length) {
      host.innerHTML = '<div class="empty-state py-3"><strong>Belum ada penjualan</strong><p>Produk terlaris akan tampil setelah transaksi.</p></div>';
      return;
    }
    host.innerHTML = rows.map(function (item, index) {
      return '<div class="rank-row"><span class="rank-index ' + (index === 0 ? 'first' : '') + '">' + String(index + 1).padStart(2, '0') + '</span><span class="rank-info"><strong>' + POSApi.escape(item.product_name) + '</strong><span>' + Number(item.quantity_sold).toLocaleString('id-ID') + ' terjual</span></span><span class="rank-amount">' + POSApi.money(item.revenue) + '</span></div>';
    }).join('');
  }
  function renderChart(rows) {
    if (!rows.length) { document.getElementById('chart-empty').classList.remove('d-none'); return; }
    if (!window.Chart) { document.getElementById('chart-empty').textContent = 'Grafik tidak tersedia. Periksa koneksi ke Chart.js.'; document.getElementById('chart-empty').classList.remove('d-none'); return; }
    const labels = rows.map(function (item) { return new Intl.DateTimeFormat('id-ID', { day: 'numeric', month: 'short' }).format(new Date(item.date + 'T00:00:00')); });
    new Chart(document.getElementById('sales-chart'), {
      type: 'line', data: { labels: labels, datasets: [{ data: rows.map(function (item) { return Number(item.revenue); }), borderColor: '#4fb3e8', backgroundColor: 'rgba(79,179,232,.12)', borderWidth: 2, fill: true, tension: .35, pointRadius: 3, pointHoverRadius: 5, pointBackgroundColor: '#fff', pointBorderWidth: 2 }] },
      options: { maintainAspectRatio: false, plugins: { legend: { display: false }, tooltip: { callbacks: { label: function (context) { return POSApi.money(context.raw); } } } }, scales: { x: { grid: { display: false }, border: { display: false }, ticks: { color: '#98a3b0', font: { family: 'Plus Jakarta Sans', size: 9 } } }, y: { beginAtZero: true, grid: { color: '#f0f3f5' }, border: { display: false }, ticks: { color: '#98a3b0', font: { family: 'Plus Jakarta Sans', size: 9 }, callback: function (value) { return 'Rp' + Number(value).toLocaleString('id-ID'); } } } } }
    });
  }
  async function load() {
    try {
      const data = await POSApi.request('/dashboard');
      const kpi = data.kpi || {};
      document.getElementById('kpi-revenue').textContent = POSApi.money(kpi.total_revenue);
      document.getElementById('kpi-transactions').textContent = Number(kpi.total_transactions || 0).toLocaleString('id-ID');
      document.getElementById('kpi-products').textContent = Number(kpi.total_products || 0).toLocaleString('id-ID');
      document.getElementById('kpi-low-stock').textContent = Number(kpi.low_stock_products || 0).toLocaleString('id-ID');
      const avg = Number(kpi.total_transactions) ? Number(kpi.total_revenue) / Number(kpi.total_transactions) : 0;
      document.getElementById('kpi-average').textContent = POSApi.money(avg);
      renderChart(data.daily_sales || []);
      renderTop(data.top_products || []);
      renderRecent(data.recent_transactions || []);
    } catch (error) {
      POSApi.showAlert(alertBox, POSApi.formatError(error));
      document.getElementById('top-products').innerHTML = '<div class="empty-state py-3"><strong>Data belum dapat dimuat</strong><p>Periksa koneksi lalu muat ulang halaman.</p></div>';
      document.getElementById('recent-transactions').innerHTML = '<tr><td colspan="6" class="text-center text-muted py-4">Data belum dapat dimuat.</td></tr>';
    }
  }
  load();
})();
