// ===== Wails 后端绑定 =====
// Wails v2 自动生成 window.go.main.App.* 方法

// ===== 状态 =====
let currentMonsterIdx = -1;
let monsters = [];
let simResult = null;
let logEntries = [];

// ===== DOM =====
const $ = id => document.getElementById(id);
const $$ = sel => document.querySelectorAll(sel);

// ===== 初始化 =====
window.addEventListener('DOMContentLoaded', () => {
  initTabs();
  initToolbar();
  initEditTab();
  initSimTab();
  initAuthTab();
  loadConfig();
});

// ===== 配置加载 =====
async function loadConfig() {
  try {
    const cfg = await window.go.main.App.GetConfig();
    if (cfg && cfg.server_root) {
      $('serverPath').value = cfg.server_root;
    }
  } catch(e) { console.log('loadConfig:', e); }
}

// ===== Tab 切换 =====
function initTabs() {
  $$('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      $$('.tab').forEach(t => t.classList.remove('active'));
      $$('.tab-pane').forEach(p => p.classList.remove('active'));
      tab.classList.add('active');
      $('pane-' + tab.dataset.tab).classList.add('active');
      $('sidebar').style.display = tab.dataset.tab === 'edit' ? 'flex' : 'none';
    });
  });
}

// ===== 工具栏 =====
function initToolbar() {
  // 浏览按钮 - 用 Wails runtime 打开系统文件夹选择对话框
  $('btnBrowse').onclick = async () => {
    try {
      const result = await window.runtime.OpenDirectoryDialog({
        Title: '选择传奇服务端根目录',
        DefaultDirectory: $('serverPath').value || ''
      });
      if (result) {
        $('serverPath').value = result;
        await window.go.main.App.SetServerPath(result);
        setStatus('已选择目录: ' + result);
      }
    } catch(e) {
      console.error('Browse error:', e);
      setStatus('浏览失败: ' + e);
    }
  };

  $('btnDetect').onclick = async () => {
    const path = $('serverPath').value;
    if (!path) return setStatus('请先输入服务端目录');
    const engine = await window.go.main.App.DetectEngine(path);
    $('engineSelect').value = engine;
    setStatus('检测到引擎: ' + engine);
  };

  $('btnLoad').onclick = async () => {
    const path = $('serverPath').value;
    if (!path) return setStatus('请选择服务端目录');
    try {
      const result = await window.go.main.App.LoadFiles(path);
      monsters = result.monsters;
      renderMonsterList();
      setStatus(`已加载 ${result.totalFiles} 个怪物文件，共 ${result.totalEntries} 条掉落配置 | 引擎: ${result.engine}`);
      if (result.warnings && result.warnings.length > 0) {
        showModal('解析提示', `<div style="max-height:400px;overflow:auto;font-family:monospace;font-size:12px;white-space:pre">${result.warnings.join('\n')}</div>`, [{text:'确定',cls:'btn-gold',action:hideModal}]);
      }
    } catch(e) { setStatus('错误: ' + e); }
  };

  $('engineSelect').onchange = () => {
    window.go.main.App.SetEngine($('engineSelect').value);
  };
}

// ===== 怪物列表 =====
function renderMonsterList() {
  const list = $('monsterList');
  list.innerHTML = '';
  monsters.forEach((m, i) => {
    const div = document.createElement('div');
    div.className = 'mon-item' + (i === currentMonsterIdx ? ' active' : '');
    div.innerHTML = `<span class="name">${m.name}</span><span class="cnt">${m.count}条</span>`;
    div.onclick = () => selectMonster(i);
    list.appendChild(div);
  });
}

async function selectMonster(idx) {
  currentMonsterIdx = idx;
  $$('.mon-item').forEach((el, i) => el.classList.toggle('active', i === idx));
  await loadEntries(idx);
}

async function loadEntries(idx) {
  try {
    const entries = await window.go.main.App.GetEntries(idx);
    renderEntries(entries);
  } catch(e) { setStatus('错误: ' + e); }
}

function renderEntries(entries) {
  const list = $('entryList');
  list.innerHTML = '';
  if (!entries) return;
  entries.forEach((e, i) => {
    const div = document.createElement('div');
    div.className = 'entry';
    const indent = '  '.repeat(e.depth);
    let icon = '🎯', cls = '', text = '';
    if (e.isComment) { icon = '📝'; cls = 'ecomm'; text = e.rawLine; }
    else if (e.isCallRef) { icon = '📎'; cls = 'ecall'; text = `#CALL [${e.callPath}]` + (e.callLabel ? ' ' + e.callLabel : ''); }
    else if (e.isChildStart) { icon = '📦'; cls = 'echild'; text = `#CHILD ${e.childProb}` + (e.childRandom ? ' RANDOM' : ''); }
    else if (e.isChildEnd) { icon = '📦'; cls = 'echild'; text = ')'; }
    else if (e.isCaseStart) { icon = '🔀'; cls = 'echild'; text = `#CASE ${e.caseExpr}`; }
    else if (e.isIfStart) { icon = '🔀'; cls = 'echild'; text = `#IF ${e.caseExpr}`; }
    else if (e.isEditable) {
      const trig = e.hasTrigger ? ` |${e.triggerName}` : '';
      text = `<span class="prob">${e.probStr}</span> <span class="iname">${e.itemName}</span><span class="trig">${trig}</span> <span class="qty">x${e.quantity}</span>`;
      div.innerHTML = `<span class="icon">${icon}</span><span>${indent}${text}</span>`;
      div.onclick = () => showEditDialog(i, e);
      list.appendChild(div);
      return;
    }
    else { text = e.rawLine; }
    div.innerHTML = `<span class="icon ${cls}">${icon}</span><span class="${cls}">${indent}${text}</span>`;
    list.appendChild(div);
  });
}

// ===== 爆率修改 =====
function initEditTab() {
  $('btnAdd').onclick = () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    showModal('新增掉落配置', `
      <div class="form-row"><label>物品名称:</label><input type="text" id="mItem" placeholder="物品名称" /></div>
      <div class="form-row"><label>概率分子:</label><input type="text" id="mNum" value="1" /></div>
      <div class="form-row"><label>概率分母:</label><input type="text" id="mDen" value="100" /></div>
      <div class="form-row"><label>掉落数量:</label><input type="text" id="mQty" value="1" /></div>
    `, [
      {text:'添加',cls:'btn-gold',action:async()=>{
        const item=$('mItem').value,num=+$('mNum').value,den=+$('mDen').value,qty=+$('mQty').value;
        if(!item||den<=0||qty<=0) return setStatus('请填写完整信息');
        await window.go.main.App.AddEntry(currentMonsterIdx,item,num,den,qty);
        hideModal(); loadEntries(currentMonsterIdx);
        addLog(`新增掉落: ${item} ${num}/${den} x${qty}`);
      }},
      {text:'取消',cls:'',action:hideModal}
    ]);
  };

  $('btnMul').onclick = () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    showModal('批量倍率调整', `<div class="form-row"><label>倍率:</label><input type="text" id="mMul" value="2.0" /></div>`, [
      {text:'执行',cls:'btn-gold',action:async()=>{
        const mul=+$('mMul').value;
        if(isNaN(mul)||mul<=0) return setStatus('请输入有效正数');
        const count=await window.go.main.App.BatchMultiply(currentMonsterIdx,mul);
        hideModal(); loadEntries(currentMonsterIdx);
        addLog(`批量倍率 x${mul}: ${count}条`);
      }},
      {text:'取消',cls:'',action:hideModal}
    ]);
  };

  $('btnSetProb').onclick = () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    showModal('批量设置概率', `
      <div class="form-row"><label>概率分子:</label><input type="text" id="mNum" value="1" /></div>
      <div class="form-row"><label>概率分母:</label><input type="text" id="mDen" value="100" /></div>
    `, [
      {text:'执行',cls:'btn-gold',action:async()=>{
        const num=+$('mNum').value,den=+$('mDen').value;
        if(den<=0) return setStatus('分母必须大于0');
        const count=await window.go.main.App.BatchSetProb(currentMonsterIdx,num,den);
        hideModal(); loadEntries(currentMonsterIdx);
        addLog(`批量设置概率 ${num}/${den}: ${count}条`);
      }},
      {text:'取消',cls:'',action:hideModal}
    ]);
  };

  $('btnAnomaly').onclick = async () => {
    const anomalies = await window.go.main.App.DetectAnomaly();
    if (!anomalies || anomalies.length === 0) return setStatus('未发现异常配置');
    const html = anomalies.map(a => `<div style="margin-bottom:4px">[${a.monster}] ${a.item||''} ${a.prob||''} — ${a.reason}</div>`).join('');
    showModal(`异常检测结果 (${anomalies.length}个)`, `<div style="max-height:400px;overflow:auto;font-size:12px">${html}</div>`, [{text:'确定',cls:'btn-gold',action:hideModal}]);
    addLog(`异常检测: 发现${anomalies.length}个异常`);
  };

  $('btnBackup').onclick = async () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    const path = await window.go.main.App.BackupCurrent(currentMonsterIdx);
    setStatus('已备份: ' + path);
    addLog('备份文件: ' + path);
  };

  $('btnBackupAll').onclick = async () => {
    const path = await window.go.main.App.BackupAll();
    setStatus('已备份目录: ' + path);
    addLog('备份目录: ' + path);
  };

  $('btnSave').onclick = async () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    await window.go.main.App.SaveFile(currentMonsterIdx);
    setStatus('已保存');
    addLog('保存文件');
  };
}

function showEditDialog(idx, entry) {
  showModal('修改掉落配置', `
    <div class="form-row"><label>物品名称:</label><span style="color:#5b9df0">${entry.itemName}</span></div>
    <div class="form-row"><label>概率分子:</label><input type="text" id="mNum" value="${entry.probNum}" /></div>
    <div class="form-row"><label>概率分母:</label><input type="text" id="mDen" value="${entry.probDen}" /></div>
    <div class="form-row"><label>掉落数量:</label><input type="text" id="mQty" value="${entry.quantity}" /></div>
  `, [
    {text:'保存',cls:'btn-gold',action:async()=>{
      const num=+$('mNum').value,den=+$('mDen').value,qty=+$('mQty').value;
      if(den<=0||qty<=0) return setStatus('请输入有效正整数');
      await window.go.main.App.ModifyEntry(currentMonsterIdx,idx,num,den,qty);
      hideModal(); loadEntries(currentMonsterIdx);
      addLog(`修改 ${entry.itemName}: ${num}/${den} x${qty}`);
    }},
    {text:'取消',cls:'',action:hideModal}
  ]);
}

// ===== 模拟 =====
function initSimTab() {
  // Radio 切换
  $$('.radio').forEach(el => {
    el.onclick = () => {
      const group = el.dataset.group;
      $$(`.radio[data-group="${group}"]`).forEach(r => r.classList.remove('selected'));
      el.classList.add('selected');
    };
  });

  $('btnSim').onclick = async () => {
    try {
      setStatus('正在模拟...');
      const req = {
        durationHours: +$('simDuration').value,
        killRatioPct: +$('simKillRatio').value,
        pityEnabled: $('simPity').checked,
        pityThreshold: +$('simPityVal').value,
        runCount: +$('simRunCount').value,
        monsters: [], items: [], maps: []
      };
      simResult = await window.go.main.App.RunSimulation(req);
      renderSimResult(simResult);
      setStatus(`模拟完成 — 击杀:${simResult.totalKills} 掉落:${simResult.totalDrops} 空爆率:${(simResult.emptyRate*100).toFixed(1)}%`);
      addLog(`模拟完成: ${simResult.totalKills}击杀 ${simResult.totalDrops}掉落`);
    } catch(e) { setStatus('模拟失败: ' + e); }
  };

  $('btnExport').onclick = async () => {
    const text = await window.go.main.App.ExportSimResult();
    if (!text) return setStatus('请先运行模拟');
    const blob = new Blob([text], {type:'text/plain'});
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = '模拟结果.txt';
    a.click();
  };

  // 搜索
  $('searchItem').oninput = e => filterTable('itemTableBody', e.target.value);
  $('searchMap').oninput = e => filterTable('mapTableBody', e.target.value);
  $('searchMon').oninput = e => filterTable('monTableBody', e.target.value);
}

function renderSimResult(r) {
  // 物品表
  renderCol('itemTableBody', ['物品名称','掉落数量'], r.itemStats.map(s=>[s.itemName, fmtNum(s.dropCount)]));
  // 地图表
  renderCol('mapTableBody', ['地图名称','掉落数量'], r.mapStats.map(s=>[s.mapName, fmtNum(s.dropCount)]));
  // 怪物表
  renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], r.monsterStats.map(s=>[s.monsterName, `${fmtNum(s.killCount)} / ${fmtNum(s.dropCount)}`]));
  // 统计
  $('sumKill').textContent = '总击杀: ' + fmtNum(r.totalKills);
  $('sumDrop').textContent = '总掉落: ' + fmtNum(r.totalDrops);
  $('sumEmpty').textContent = '空爆率: ' + (r.emptyRate*100).toFixed(1) + '%';
  $('sumTypes').textContent = '物品种类: ' + r.itemStats.length;
  // 稀有追踪
  const rare = r.itemStats.filter(s => s.prob > 0 && s.prob < 0.001).slice(0, 10);
  $('trackerItems').innerHTML = rare.map(s => `<span class="tracker-item">${s.itemName} ×${fmtNum(s.dropCount)} (${(s.prob*100).toFixed(3)}%)</span>`).join('');
}

function renderCol(bodyId, headers, rows) {
  const body = $(bodyId);
  let html = `<div class="rrow hdr"><span class="lbl">${headers[0]}</span><span class="val">${headers[1]}</span></div>`;
  rows.forEach(r => {
    html += `<div class="rrow"><span class="lbl">${r[0]}</span><span class="val">${r[1]}</span></div>`;
  });
  body.innerHTML = html;
}

function filterTable(bodyId, query) {
  const rows = $(bodyId).querySelectorAll('.rrow:not(.hdr)');
  const q = query.toLowerCase();
  rows.forEach(r => {
    const text = r.textContent.toLowerCase();
    r.style.display = text.includes(q) ? '' : 'none';
  });
}

// ===== 授权 =====
function initAuthTab() {
  $('btnCopyCode').onclick = async () => {
    const code = await window.go.main.App.GetMachineID();
    $('machineCode').textContent = code;
    navigator.clipboard.writeText(code);
    setStatus('机器码已复制');
  };

  $('btnActivate').onclick = async () => {
    const code = $('activateCode').value.trim();
    if (!code) return setStatus('请输入激活码');
    try {
      const info = await window.go.main.App.Activate(code);
      $('licenseStatus').textContent = '✅ 已激活 (' + info.type + ')';
      $('licenseStatus').className = 'status-badge active';
      setStatus('激活成功');
    } catch(e) { setStatus('激活失败: ' + e); }
  };

  // 加载授权状态
  loadLicenseStatus();
}

async function loadLicenseStatus() {
  try {
    const info = await window.go.main.App.GetLicenseStatus();
    if (info.isActive) {
      $('licenseStatus').textContent = '✅ 已激活 (' + info.type + ')';
      $('licenseStatus').className = 'status-badge active';
    } else {
      $('licenseStatus').textContent = '未激活';
      $('licenseStatus').className = 'status-badge inactive';
    }
    const code = await window.go.main.App.GetMachineID();
    $('machineCode').textContent = code;
  } catch(e) {}
}

// ===== 工具函数 =====
function setStatus(text) { $('statusText').textContent = text; }
function fmtNum(n) { return n.toLocaleString(); }
function addLog(text) {
  const time = new Date().toLocaleTimeString('zh-CN', {hour12:false});
  logEntries.unshift(`[${time}] ${text}`);
  if (logEntries.length > 200) logEntries.pop();
  $('logArea').innerHTML = logEntries.join('<br/>');
}

// ===== 模态框 =====
function showModal(title, bodyHtml, buttons) {
  $('modalTitle').textContent = title;
  $('modalBody').innerHTML = bodyHtml;
  $('modalFooter').innerHTML = '';
  buttons.forEach(b => {
    const btn = document.createElement('button');
    btn.className = 'btn ' + (b.cls || '');
    btn.textContent = b.text;
    btn.onclick = b.action;
    $('modalFooter').appendChild(btn);
  });
  $('modalOverlay').classList.add('show');
}
function hideModal() { $('modalOverlay').classList.remove('show'); }
$('modalClose').onclick = hideModal;
$('modalOverlay').onclick = e => { if (e.target === $('modalOverlay')) hideModal(); };