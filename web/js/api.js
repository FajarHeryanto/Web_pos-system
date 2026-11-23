(function () {
  'use strict';
  const API_BASE = String(window.POS_API_BASE || '').replace(/\/$/, '');
  const TOKEN_KEY = 'pos_token';
  const USER_KEY = 'pos_user';

  function getToken() { return localStorage.getItem(TOKEN_KEY) || ''; }
  function getUser() {
    try { return JSON.parse(localStorage.getItem(USER_KEY) || 'null'); }
    catch (_) { return null; }
  }
  function saveSession(token, user) {
    localStorage.setItem(TOKEN_KEY, token);
    localStorage.setItem(USER_KEY, JSON.stringify(user));
  }
  function clearSession() {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  }
  function redirectLogin() {
    if (!location.pathname.startsWith('/login')) location.replace('/login');
  }
  async function request(path, options) {
    const opts = Object.assign({ method: 'GET', headers: {} }, options || {});
    opts.headers = Object.assign({ Accept: 'application/json' }, opts.headers || {});
    if (opts.body && typeof opts.body !== 'string') {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(opts.body);
    }
    if (!opts.public && getToken()) opts.headers.Authorization = 'Bearer ' + getToken();
    let response;
    try {
      response = await fetch(API_BASE + '/api' + path, opts);
    } catch (error) {
      const failure = new Error('Tidak dapat terhubung ke server. Periksa koneksi lalu coba lagi.');
      failure.cause = error;
      throw failure;
    }
    let payload;
    try { payload = await response.json(); }
    catch (_) { payload = { status: 'error', message: 'Server mengirim respons yang tidak dapat dibaca.' }; }
    if (!response.ok || payload.status === 'error') {
      if (response.status === 401 && !opts.public) {
        clearSession();
        redirectLogin();
      }
      const failure = new Error(payload.message || 'Permintaan tidak berhasil.');
      failure.status = response.status;
      failure.errors = payload.errors || [];
      failure.payload = payload;
      throw failure;
    }
    return payload.data;
  }
  function money(value) {
    return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(Number(value) || 0);
  }
  function date(value) {
    if (!value) return '—';
    const parsed = new Date(value + (String(value).length === 10 ? 'T00:00:00' : ''));
    return Number.isNaN(parsed.getTime()) ? String(value) : new Intl.DateTimeFormat('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }).format(parsed);
  }
  function escape(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, function (ch) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch];
    });
  }
  function initials(name) {
    return String(name || 'U').trim().split(/\s+/).slice(0, 2).map(function (part) { return part[0] || ''; }).join('').toUpperCase();
  }
  function showAlert(target, message, type) {
    const host = typeof target === 'string' ? document.querySelector(target) : target;
    if (!host) return;
    host.innerHTML = '<div class="alert alert-' + (type || 'danger') + ' alert-dismissible fade show" role="alert">' + escape(message) + '<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Tutup"></button></div>';
  }
  function formatError(error) {
    if (error && Array.isArray(error.errors) && error.errors.length) {
      return error.errors.map(function (item) { return item.field + ': ' + item.message; }).join(' · ');
    }
    return error && error.message ? error.message : 'Terjadi kesalahan. Silakan coba lagi.';
  }
  window.POSApi = { request, getToken, getUser, saveSession, clearSession, redirectLogin, money, date, escape, initials, showAlert, formatError };
})();
