(function () {
  'use strict';
  const runC = document.getElementById('run-c');
  const runCobol = document.getElementById('run-cobol');
  const output = document.getElementById('report-output');
  let hasProcessed = false;

  function setStep(index, state, message) {
    const number = document.getElementById('step-number-' + index);
    const status = document.getElementById('step-status-' + index);
    number.className = 'step-number' + (state === 'complete' ? ' complete' : state === 'running' ? ' running' : '');
    status.className = 'step-status' + (state === 'complete' ? ' complete' : state === 'failed' ? ' failed' : '');
    status.textContent = message;
    if (state === 'complete') number.innerHTML = '<i class="bi bi-check2"></i>';
    else number.textContent = String(index);
  }
  function setBusy(button, busy, label) {
    button.disabled = busy;
    if (busy) button.dataset.original = button.innerHTML;
    button.innerHTML = busy ? '<span class="spinner-border spinner-border-sm me-2"></span>' + label : button.dataset.original || label;
  }
  async function loadReport() {
    try {
      const data = await POSApi.request('/reports/income');
      output.textContent = data.report || 'Laporan kosong.';
      output.classList.add('has-report');
      document.getElementById('report-meta').textContent = 'Laporan dibuat, ' + (Number(data.bytes) || 0).toLocaleString('id-ID') + ' byte.';
      setStep(2, 'complete', 'Laporan tersedia');
    } catch (error) {
      if (error.status === 404) {
        output.textContent = 'Belum ada laporan. Jalankan C Engine lalu COBOL Reporter untuk membuat laporan.';
        output.classList.remove('has-report');
        return;
      }
      POSApi.showAlert('#report-alert', POSApi.formatError(error));
    }
  }
  runC.addEventListener('click', async function () {
    POSApi.showAlert('#report-alert', '', 'info');
    document.getElementById('report-alert').innerHTML = '';
    setStep(1, 'running', 'Sedang memproses data transaksi…');
    setBusy(runC, true, 'Memproses…');
    runCobol.disabled = true;
    try {
      const result = await POSApi.request('/process', { method: 'POST' });
      hasProcessed = true;
      const stats = result.statistics || {};
      setStep(1, 'complete', Number(result.record_count || 0).toLocaleString('id-ID') + ' transaksi diproses');
      runCobol.disabled = false;
      output.textContent = 'C Engine berhasil memproses ' + Number(result.record_count || 0).toLocaleString('id-ID') + ' transaksi.\n\nRingkasan kalkulasi:\n' + Object.entries(stats).map(function (entry) { return '  ' + entry[0] + ': ' + entry[1]; }).join('\n') + '\n\nJalankan COBOL Reporter untuk menyusun laporan akhir.';
      output.classList.add('has-report');
      document.getElementById('report-meta').textContent = 'Data CSV berhasil diperbarui. Laporan menunggu COBOL Reporter.';
    } catch (error) {
      setStep(1, 'failed', 'Pemrosesan gagal');
      POSApi.showAlert('#report-alert', POSApi.formatError(error));
    } finally {
      setBusy(runC, false, 'Jalankan C Engine');
      runC.disabled = false;
    }
  });
  runCobol.addEventListener('click', async function () {
    if (!hasProcessed) return;
    setStep(2, 'running', 'Sedang menyusun laporan…');
    setBusy(runCobol, true, 'Membuat laporan…');
    try {
      await POSApi.request('/reports/generate', { method: 'POST' });
      await loadReport();
      POSApi.showAlert('#report-alert', 'Laporan berhasil dibuat.', 'success');
    } catch (error) {
      setStep(2, 'failed', 'Pembuatan laporan gagal');
      POSApi.showAlert('#report-alert', POSApi.formatError(error));
    } finally {
      setBusy(runCobol, false, 'Generate COBOL');
      runCobol.disabled = !hasProcessed;
    }
  });
  document.getElementById('refresh-report').addEventListener('click', loadReport);
  document.getElementById('print-report').addEventListener('click', function () {
    if (!output.classList.contains('has-report')) { POSApi.showAlert('#report-alert', 'Belum ada laporan untuk dicetak.', 'warning'); return; }
    window.print();
  });
  if (!POSIsAdmin) {
    runC.disabled = true;
    runC.title = 'Hanya admin yang dapat menjalankan proses laporan.';
    runCobol.disabled = true;
  }
  loadReport();
})();
