/**
 * FamilySync Parent Dashboard — app.js
 * Handles: Auth, REST API calls, WebSocket live feed, GPS map, notification log.
 */
'use strict';

// ══════════════════════════════════════════════
//  Configuration
// ══════════════════════════════════════════════
const API_BASE = window.location.origin; // Same host — served by Go backend

// ══════════════════════════════════════════════
//  State
// ══════════════════════════════════════════════
let jwt = localStorage.getItem('fs_token') || '';
let selectedDevice = null;       // { id, device_name }
let devices = [];
let parentWS = null;
let mapInstance = null;
let mapMarker = null;
let isStreaming = { camera: false, screen: false, mic: false };
let reconnectTimer = null;
let intentionalDisconnect = false;

// Canvas FPS counter
let frameCount = 0;
let fpsTime = performance.now();

// ══════════════════════════════════════════════
//  Helpers
// ══════════════════════════════════════════════
const $ = id => document.getElementById(id);

function showScreen(id) {
  document.querySelectorAll('.screen').forEach(s => {
    if (s.id === id) {
      s.classList.add('active');
      s.style.display = 'flex';
    } else {
      s.classList.remove('active');
      s.style.display = 'none';
    }
  });
}

async function apiFetch(path, method = 'GET', body = null, auth = true) {
  const headers = { 'Content-Type': 'application/json' };
  if (auth && jwt) headers['Authorization'] = `Bearer ${jwt}`;
  const opts = { method, headers };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(API_BASE + path, opts);
  const data = await res.json().catch(() => ({}));
  return { ok: res.ok, status: res.status, data };
}

function showError(elemId, msg) {
  const el = $(elemId);
  el.textContent = msg;
  el.classList.remove('hidden');
}
function clearError(elemId) { $(elemId).classList.add('hidden'); }

// ══════════════════════════════════════════════
//  Auth — Login / Register
// ══════════════════════════════════════════════
$('show-register').addEventListener('click', e => {
  e.preventDefault(); showScreen('register-screen');
});
$('show-login').addEventListener('click', e => {
  e.preventDefault(); showScreen('login-screen');
});

$('login-form').addEventListener('submit', async e => {
  e.preventDefault();
  clearError('login-error');
  $('login-btn').disabled = true;
  $('login-btn').textContent = 'Signing in…';

  const email = $('login-email').value;
  const password = $('login-password').value;
  const { ok, data } = await apiFetch('/api/v1/auth/login', 'POST', { email, password }, false);

  $('login-btn').disabled = false;
  $('login-btn').textContent = 'Sign In';

  if (ok && data.token) {
    jwt = data.token;
    localStorage.setItem('fs_token', jwt);
    await loadDashboard();
  } else {
    showError('login-error', data.error || 'Login failed');
  }
});

$('register-form').addEventListener('submit', async e => {
  e.preventDefault();
  clearError('reg-error');
  $('register-btn').disabled = true;

  const email = $('reg-email').value;
  const password = $('reg-password').value;
  const { ok, data } = await apiFetch('/api/v1/auth/register', 'POST', { email, password }, false);

  $('register-btn').disabled = false;

  if (ok && data.token) {
    jwt = data.token;
    localStorage.setItem('fs_token', jwt);
    await loadDashboard();
  } else {
    showError('reg-error', data.error || 'Registration failed');
  }
});

$('logout-btn').addEventListener('click', performLogout);
if ($('logout-btn-mobile')) $('logout-btn-mobile').addEventListener('click', performLogout);

function performLogout() {
  jwt = '';
  localStorage.removeItem('fs_token');
  disconnectParentWS();
  if (window.statusPollTimer) {
    clearInterval(window.statusPollTimer);
    window.statusPollTimer = null;
  }
  showScreen('login-screen');
}

// ══════════════════════════════════════════════
//  Dashboard — Load devices
// ══════════════════════════════════════════════
async function loadDashboard() {
  showScreen('dashboard-screen');
  const { ok, data } = await apiFetch('/api/v1/device/list');
  if (!ok) { jwt = ''; showScreen('login-screen'); return; }

  devices = data.devices || [];
  renderDeviceNav();
  await refreshStatuses();
  
  // Auto-refresh ping agar indikator abu-abu/hijau terupdate real-time setiap 10 detik
  if (!window.statusPollTimer) {
    window.statusPollTimer = setInterval(async () => {
      await refreshStatuses();
      updateOnlineBadge();
    }, 10000);
  }
}

function renderDeviceNav() {
  const nav = $('device-nav');
  nav.innerHTML = `<div class="nav-section-label">Devices (${devices.length})</div>`;

  if (devices.length === 0) {
    nav.innerHTML += `<div style="padding:.8rem 1.2rem;color:var(--muted);font-size:.85rem">
      No devices paired yet.</div>`;
    return;
  }

  devices.forEach(dev => {
    const item = document.createElement('div');
    item.className = 'nav-device-item';
    item.id = `nav-${dev.id}`;
    item.innerHTML = `
      <span class="nav-device-dot" id="dot-${dev.id}"></span>
      <span>${dev.device_name}</span>`;
    item.addEventListener('click', () => selectDevice(dev));
    nav.appendChild(item);
  });
}

async function refreshStatuses() {
  const { ok, data } = await apiFetch('/api/v1/device/status');
  if (!ok) return;
  (data.devices || []).forEach(d => {
    const dot = $(`dot-${d.id}`);
    if (dot) dot.classList.toggle('online', d.online);
  });
}

// ══════════════════════════════════════════════
//  Device Selection
// ══════════════════════════════════════════════
function selectDevice(dev) {
  selectedDevice = dev;

  // Highlight nav
  document.querySelectorAll('.nav-device-item')
    .forEach(el => el.classList.remove('active'));
  const navItem = $(`nav-${dev.id}`);
  if (navItem) navItem.classList.add('active');

  // Show panel
  $('empty-state').classList.add('hidden');
  $('device-panel').classList.remove('hidden');
  $('panel-device-name').textContent = dev.device_name;

  updateOnlineBadge();
  disconnectParentWS();
  connectParentWS(dev.id);
  resetStreamButtons();
}

function updateOnlineBadge() {
  if (!selectedDevice) return;
  const dot = $(`dot-${selectedDevice.id}`);
  const online = dot?.classList.contains('online');
  const badge = $('panel-status-badge');
  badge.textContent = online ? '● Online' : '● Offline';
  badge.className = 'status-badge ' + (online ? 'online' : 'offline');
}

$('refresh-status-btn').addEventListener('click', async () => {
  await refreshStatuses();
  updateOnlineBadge();
});

// Delete Device Button logic
$('delete-device-btn')?.addEventListener('click', async () => {
  if (!selectedDevice) return;
  const confirmDelete = confirm(`⚠️ Are you sure you want to DELETE "${selectedDevice.device_name}"?\n\nThis will completely unpair the device from your account. The child's phone app will need to be re-paired manually if you want to monitor it again.`);
  
  if (confirmDelete) {
    const { ok, data } = await apiFetch(`/api/v1/device/${selectedDevice.id}`, 'DELETE');
    if (ok) {
      alert("Device successfully removed!");
      selectedDevice = null;
      $('device-panel').classList.add('hidden');
      $('empty-state').classList.remove('hidden');
      await loadDashboard(); // refresh left sidebar
    } else {
      alert(`Delete failed: ${data.error || 'Server error'}`);
    }
  }
});

// ══════════════════════════════════════════════
//  Command Dispatch
// ══════════════════════════════════════════════
async function sendCommand(action, params = {}) {
  if (!selectedDevice) return;
  const { ok, data } = await apiFetch(
    `/api/v1/command/${selectedDevice.id}`, 'POST', { action, params }
  );
  if (!ok) {
    alert(`Command failed: ${data.error || 'Device offline?'}`);
  }
  return ok;
}

// GPS button
$('btn-gps').addEventListener('click', async () => {
  $('btn-gps').textContent = 'Requesting…';
  await sendCommand('GET_GPS');
  $('btn-gps').textContent = 'Get GPS Now';
  // Result arrives via parent WebSocket → handleFrame()
});

// Toggle buttons (camera / screen / mic)
function setupToggleBtn(btnId, startAction, stopAction, streamKey, indicatorId, label) {
  $(btnId).addEventListener('click', async () => {
    const streaming = isStreaming[streamKey];
    const action = streaming ? stopAction : startAction;
    
    // Eagerly hide modal and block incoming zombie frames to prevent UI judder
    if (streaming) {
      if (streamKey === 'camera') {
        window.closing_camera = true;
        $('feed-modal').classList.add('hidden');
      }
      if (streamKey === 'screen') {
        window.closing_screen = true;
        $('feed-modal').classList.add('hidden');
      }
    }

    const ok = await sendCommand(action);
    if (!ok) {
      // Revert flags if network request failed completely
      if (streamKey === 'camera') window.closing_camera = false;
      if (streamKey === 'screen') window.closing_screen = false;
      return;
    }

    isStreaming[streamKey] = !streaming;
    const btn = $(btnId);
    btn.textContent = isStreaming[streamKey] ? `Stop ${label}` : `Start ${label}`;
    btn.classList.toggle('active', isStreaming[streamKey]);
    const ind = $(indicatorId);
    if (ind) ind.classList.toggle('active', isStreaming[streamKey]);

    // Clear closing flag once command finishes
    if (streamKey === 'camera') window.closing_camera = false;
    if (streamKey === 'screen') window.closing_screen = false;

    if (streamKey === 'mic') {
      $('mic-activity').classList.toggle('hidden', !isStreaming[streamKey]);
    }

    if (!isStreaming[streamKey] && (streamKey === 'camera' || streamKey === 'screen')) {
      $('feed-modal').classList.add('hidden');
    }

    // Auto-sync Mic with Camera
    if (streamKey === 'camera') {
      if (isStreaming['camera'] !== isStreaming['mic']) {
        $('btn-mic').click(); // trigger macro
      }
    }
  });
}

setupToggleBtn('btn-camera', 'START_CAMERA', 'STOP_CAMERA', 'camera', 'camera-indicator', 'Camera');
setupToggleBtn('btn-screen', 'START_SCREEN', 'STOP_SCREEN', 'screen', 'screen-indicator', 'Screen');
setupToggleBtn('btn-mic', 'START_MIC', 'STOP_MIC', 'mic', 'mic-indicator', 'Mic');

function resetStreamButtons() {
  isStreaming = { camera: false, screen: false, mic: false };
  ['btn-camera', 'btn-screen', 'btn-mic'].forEach(id => {
    const btn = $(id);
    btn.classList.remove('active');
  });
  $('btn-camera').textContent = 'Start Camera';
  $('btn-screen').textContent = 'Start Screen';
  $('btn-mic').textContent = 'Start Mic';
  ['camera-indicator', 'screen-indicator', 'mic-indicator'].forEach(id => {
    $(id)?.classList.remove('active');
  });
  $('mic-activity').classList.add('hidden');
  $('feed-modal').classList.add('hidden');
}

// Modal Close logic
const btnGpsClose = $('close-gps-modal');
if (btnGpsClose) {
  btnGpsClose.addEventListener('click', () => {
    $('gps-modal').classList.add('hidden');
  });
}

const btnFeedClose = $('close-feed-modal');
if (btnFeedClose) {
  btnFeedClose.addEventListener('click', () => {
    $('feed-modal').classList.add('hidden');
    // Auto-stop stream when modal is closed
    if (isStreaming.camera) {
      window.closing_camera = true;
      $('btn-camera').click();
    }
    if (isStreaming.screen) {
      window.closing_screen = true;
      $('btn-screen').click();
    }
  });
}

// ══════════════════════════════════════════════
//  Parent WebSocket — Live Feed Receiver
// ══════════════════════════════════════════════
function connectParentWS(deviceId) {
  intentionalDisconnect = false;
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const url = `${proto}://${location.host}/api/v1/ws/parent/${deviceId}?token=${jwt}`;
  parentWS = new WebSocket(url);

  parentWS.binaryType = 'arraybuffer';

  parentWS.onopen = () => {
    console.log(`[WS] Parent connected for device ${deviceId}`);
  };
  
  parentWS.onclose = () => {
    console.log(`[WS] Parent disconnected`);
    parentWS = null;
    if (!intentionalDisconnect && selectedDevice && selectedDevice.id === deviceId) {
      console.log(`[WS] Connection lost. Reconnecting in 3 seconds...`);
      clearTimeout(reconnectTimer);
      reconnectTimer = setTimeout(() => connectParentWS(deviceId), 3000);
    }
  };
  
  parentWS.onerror = err => console.warn('[WS] Error:', err);
  parentWS.onmessage = evt => handleFrame(evt.data);
}

function disconnectParentWS() {
  intentionalDisconnect = true;
  clearTimeout(reconnectTimer);
  if (parentWS) { parentWS.close(); parentWS = null; }
}

function handleFrame(raw) {
  let json;
  try {
    const text = typeof raw === 'string' ? raw : new TextDecoder().decode(raw);
    json = JSON.parse(text);
  } catch {
    // Could be raw binary — ignore non-JSON
    return;
  }

  switch (json.type) {
    case 'gps':
      handleGPS(json);
      break;
    case 'camera_frame':
    case 'screen_frame':
      handleVideoFrame(json);
      break;
    case 'audio_chunk':
      handleAudioChunk(json);
      break;
  }
}

// ── GPS ──────────────────────────────────────

function handleGPS(data) {
  const lat = data.lat, lng = data.lng;
  const ts = new Date(data.timestamp).toLocaleTimeString();

  // Show result text
  const result = $('gps-result');
  result.textContent = `📍 ${lat.toFixed(6)}, ${lng.toFixed(6)}  •  ${ts}`;
  result.classList.remove('hidden');

  // Show / update map
  $('gps-modal').classList.remove('hidden');
  $('modal-gps-coords').textContent = `Lat: ${lat.toFixed(6)} | Lng: ${lng.toFixed(6)}`;

  if (!mapInstance) {
    mapInstance = L.map('map').setView([lat, lng], 15);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© OpenStreetMap'
    }).addTo(mapInstance);
    mapMarker = L.marker([lat, lng]).addTo(mapInstance);
  } else {
    mapInstance.setView([lat, lng], 15);
    mapMarker.setLatLng([lat, lng]);
  }

  // Fix Leaflet rendering bug when container was previously display: none
  setTimeout(() => mapInstance.invalidateSize(), 100);
}

// ── Video frames (camera / screen) ──────────

const canvas = $('live-canvas');
const ctx = canvas.getContext('2d');

let isFrontCamera = false;

function handleVideoFrame(data) {
  // Ignore "zombie" frames that arrive while the stop command is still travelling or after it has officially stopped.
  if (data.type === 'camera_frame' && (window.closing_camera || !isStreaming.camera)) return;
  if (data.type === 'screen_frame' && (window.closing_screen || !isStreaming.screen)) return;

  $('feed-modal').classList.remove('hidden');
  $('feed-modal-title').textContent = data.type === 'camera_frame' ? '📷 Camera Live Feed' : '🖥️ Screen Live Feed';

  if (data.type === 'camera_frame') {
    $('btn-switch-camera').classList.remove('hidden');
    canvas.style.transform = isFrontCamera ? 'rotate(180deg)' : 'rotate(90deg)';
  } else {
    $('btn-switch-camera').classList.add('hidden');
    canvas.style.transform = 'none';
  }

  const img = new Image();
  img.onload = () => {
    canvas.width = img.naturalWidth || canvas.width;
    canvas.height = img.naturalHeight || canvas.height;
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
    updateFPS();
  };
  img.onerror = () => {
    console.error(`Failed to load ${data.type} image from Base64 data (length: ${data.data?.length})`);
  };
  img.src = `data:image/jpeg;base64,${data.data}`;
}

const btnSwitchCamera = $('btn-switch-camera');
if (btnSwitchCamera) {
  btnSwitchCamera.addEventListener('click', () => {
    isFrontCamera = !isFrontCamera;
    sendCommand('SWITCH_CAMERA');
  });
}

function updateFPS() {
  frameCount++;
  const now = performance.now();
  if (now - fpsTime >= 1000) {
    $('feed-fps').textContent = `${frameCount} fps`;
    frameCount = 0;
    fpsTime = now;
  }
}

// ── Audio ─────────────────────────────────────

let audioCtx = null;

function handleAudioChunk(data) {
  // Animate mic bar (simple activity indicator)
  const bar = $('mic-bar');
  if (bar) {
    bar.style.width = '100%';
    setTimeout(() => { bar.style.width = '0'; }, 400);
  }

  // Play audio using Web Audio API
  if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();

  try {
    const binaryStr = atob(data.data);
    const len = binaryStr.length;
    // Android sends 16-bit PCM. 2 bytes per sample.
    const samples = len / 2;
    const audioBuffer = audioCtx.createBuffer(1, samples, 16000); // Mono, 16kHz
    const channelData = audioBuffer.getChannelData(0);

    let offset = 0;
    for (let i = 0; i < samples; i++) {
      // 16-bit little endian PCM to Float32 [-1, 1]
      let pcm = (binaryStr.charCodeAt(offset) & 0xff) | ((binaryStr.charCodeAt(offset + 1) & 0xff) << 8);
      if (pcm > 32767) pcm -= 65536;
      channelData[i] = pcm / 32768.0;
      offset += 2;
    }

    const source = audioCtx.createBufferSource();
    source.buffer = audioBuffer;
    source.connect(audioCtx.destination);
    source.start();
  } catch (e) {
    console.error('Audio playback error:', e);
  }
}

// ══════════════════════════════════════════════
//  Notification Log
// ══════════════════════════════════════════════
$('refresh-notifs-btn').addEventListener('click', loadNotifications);

async function loadNotifications() {
  if (!selectedDevice) return;
  const { ok, data } = await apiFetch(`/api/v1/device/${selectedDevice.id}/notifications`);
  if (!ok) return;

  const tbody = $('notif-tbody');
  const notifs = data.notifications || [];

  if (notifs.length === 0) {
    tbody.innerHTML = '<tr><td colspan="3" class="empty-row">No notifications yet.</td></tr>';
    return;
  }

  tbody.innerHTML = notifs.map(n => `
    <tr>
      <td><code>${escapeHtml(n.app_package)}</code></td>
      <td>${escapeHtml(n.content)}</td>
      <td>${new Date(n.received_at).toLocaleString()}</td>
    </tr>
  `).join('');
}

function escapeHtml(str) {
  return str.replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[c]));
}

// ══════════════════════════════════════════════
//  Init — Auto-login if JWT stored
// ══════════════════════════════════════════════
(async () => {
  if (jwt) {
    await loadDashboard();
  } else {
    showScreen('login-screen');
  }
})();
