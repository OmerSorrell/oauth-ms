(async () => {
  const $ = (sel) => document.querySelector(sel);
  const showError = (msg) => {
    const el = $('#error');
    el.textContent = msg;
    el.hidden = false;
  };

  try {
    const res = await fetch('/profile', { credentials: 'same-origin' });
    if (res.status === 401) {
      // Not signed in yet — leave the landing CTA visible.
      return;
    }
    if (!res.ok) {
      showError(`profile failed: ${res.status} ${res.statusText}`);
      return;
    }

    const data = await res.json();
    $('#signed-out').hidden = true;
    $('#signed-in').hidden = false;

    const u = data.user || {};
    $('#user-name').textContent = u.name || '(no name)';
    $('#user-license').textContent = u.license ? `Tier: ${u.license}` : '';
    $('#user-role').textContent = u.role ? `Role: ${u.role}` : '';
    $('#user-id').textContent = u.id ? `id: ${u.id}` : '';

    const tbody = $('#repos tbody');
    tbody.innerHTML = '';
    for (const r of (data.resources || [])) {
      const tr = document.createElement('tr');
      const nameTd = document.createElement('td');
      nameTd.textContent = r.name;
      const descTd = document.createElement('td');
      descTd.textContent = r.description || '';
      const metricTd = document.createElement('td');
      metricTd.className = 'stars';
      metricTd.textContent = r.metric ?? 0;
      tr.appendChild(nameTd);
      tr.appendChild(descTd);
      tr.appendChild(metricTd);
      tbody.appendChild(tr);
    }
  } catch (err) {
    showError(`network error: ${err.message}`);
  }

  $('#logout').addEventListener('click', async () => {
    await fetch('/logout', { method: 'POST', credentials: 'same-origin' });
    window.location.reload();
  });
})();
