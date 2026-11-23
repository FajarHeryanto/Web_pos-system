(function () {
  'use strict';
  if (POSApi.getToken()) location.replace('/');
  const form = document.getElementById('login-form');
  const errorBox = document.getElementById('login-error');
  const submit = form.querySelector('button[type="submit"]');
  form.addEventListener('submit', async function (event) {
    event.preventDefault();
    errorBox.classList.add('d-none');
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value;
    submit.disabled = true;
    submit.querySelector('.spinner-border').classList.remove('d-none');
    submit.querySelector('.submit-label').textContent = 'Memeriksa…';
    try {
      const data = await POSApi.request('/auth/login', { method: 'POST', body: { username, password }, public: true });
      POSApi.saveSession(data.token, data.user);
      location.replace('/');
    } catch (error) {
      errorBox.textContent = POSApi.formatError(error);
      errorBox.classList.remove('d-none');
      document.getElementById('password').select();
    } finally {
      submit.disabled = false;
      submit.querySelector('.spinner-border').classList.add('d-none');
      submit.querySelector('.submit-label').textContent = 'Masuk';
    }
  });
})();
