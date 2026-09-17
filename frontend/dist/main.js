// ===== Wails 后端绑定 =====
// Wails v2 自动生成 window.go.app.App.* 方法

// ===== 状态 =====
let currentMonsterIdx = -1;
let monsters = [];
let simResult = null;
let logEntries = [];

// ===== DOM =====
const $ = id => document.getElementById(id);
const $$ = sel => document.querySelectorAll(sel);

// ===== 初始化 =====
// 等待 Wails 绑定就绪（window.go 可能由运行时异步注入）
function waitForWailsReady(maxWaitMs = 5000) {
  return new Promise((resolve, reject) => {
    if (window.go && window.go.app && window.go.app.App) return resolve();
    const start = Date.now();
    const timer = setInterval(() => {
      if (window.go && window.go.app && window.go.app.App) {
        clearInterval(timer);
        resolve();
      } else if (Date.now() - start > maxWaitMs) {
        clearInterval(timer);
        reject(new Error('Wails 绑定加载超时'));
      }
    }, 50);
  });
}

window.addEventListener('DOMContentLoaded', async () => {
  try {
    await waitForWailsReady();
  } catch(e) {
    setStatus('错误: ' + e.message + ' — 请确认使用最新版 exe');
    return;
  }
  initTabs();
  initToolbar();
  initEditTab();
  initSimTab();
  initTuneTab();
  initAuthTab();
  loadConfig();
});

// ===== 配置加载 =====
async function loadConfig() {
  try {
    const cfg = await window.go.app.App.GetConfig();
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
  // 浏览按钮 - 通过 Go 后端打开系统目录选择对话框
  $('btnBrowse').onclick = async () => {
    try {
      const result = await window.go.app.App.SelectDirectory();
      if (result) {
        $('serverPath').value = result;
        await window.go.app.App.SetServerPath(result);
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
    const engine = await window.go.app.App.DetectEngine(path);
    $('engineSelect').value = engine;
    setStatus('检测到引擎: ' + engine);
  };

  $('btnLoad').onclick = async () => {
    const path = $('serverPath').value;
    if (!path) return setStatus('请选择服务端目录');
    try {
      const result = await window.go.app.App.LoadFiles(path);
      monsters = result.monsters;
      renderMonsterList();
      refreshTuneItemList();
      setStatus(`已加载 ${result.totalFiles} 个怪物文件，共 ${result.totalEntries} 条掉落配置 | 引擎: ${result.engine}`);
      if (result.warnings && result.warnings.length > 0) {
        showModal('解析提示', `<div style="max-height:400px;overflow:auto;font-family:monospace;font-size:12px;white-space:pre">${result.warnings.join('\n')}</div>`, [{text:'确定',cls:'btn-gold',action:hideModal}]);
      }
    } catch(e) { setStatus('错误: ' + e); }
  };

  $('engineSelect').onchange = () => {
    window.go.app.App.SetEngine($('engineSelect').value);
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
    const entries = await window.go.app.App.GetEntries(idx);
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
        await window.go.app.App.AddEntry(currentMonsterIdx,item,num,den,qty);
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
        const count=await window.go.app.App.BatchMultiply(currentMonsterIdx,mul);
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
        const count=await window.go.app.App.BatchSetProb(currentMonsterIdx,num,den);
        hideModal(); loadEntries(currentMonsterIdx);
        addLog(`批量设置概率 ${num}/${den}: ${count}条`);
      }},
      {text:'取消',cls:'',action:hideModal}
    ]);
  };

  $('btnAnomaly').onclick = async () => {
    const anomalies = await window.go.app.App.DetectAnomaly();
    if (!anomalies || anomalies.length === 0) return setStatus('未发现异常配置');
    const html = anomalies.map(a => `<div style="margin-bottom:4px">[${a.monster}] ${a.item||''} ${a.prob||''} — ${a.reason}</div>`).join('');
    showModal(`异常检测结果 (${anomalies.length}个)`, `<div style="max-height:400px;overflow:auto;font-size:12px">${html}</div>`, [{text:'确定',cls:'btn-gold',action:hideModal}]);
    addLog(`异常检测: 发现${anomalies.length}个异常`);
  };

  $('btnBackup').onclick = async () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    const path = await window.go.app.App.BackupCurrent(currentMonsterIdx);
    setStatus('已备份: ' + path);
    addLog('备份文件: ' + path);
  };

  $('btnBackupAll').onclick = async () => {
    const path = await window.go.app.App.BackupAll();
    setStatus('已备份目录: ' + path);
    addLog('备份目录: ' + path);
  };

  $('btnSave').onclick = async () => {
    if (currentMonsterIdx < 0) return setStatus('请先选择怪物');
    await window.go.app.App.SaveFile(currentMonsterIdx);
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
      await window.go.app.App.ModifyEntry(currentMonsterIdx,idx,num,den,qty);
      hideModal(); loadEntries(currentMonsterIdx);
      addLog(`修改 ${entry.itemName}: ${num}/${den} x${qty}`);
    }},
    {text:'取消',cls:'',action:hideModal}
  ]);
}

// ===== 筛选状态 =====
let filterMonsters = [];
let filterItems = [];
let filterMaps = [];

// ===== 模拟 =====
function initSimTab() {
  // Radio 切换 + 指定选择弹窗
  $$('.radio').forEach(el => {
    el.onclick = async () => {
      const group = el.dataset.group;
      const val = el.dataset.val;
      $$(`.radio[data-group="${group}"]`).forEach(r => r.classList.remove('selected'));
      el.classList.add('selected');

      // 点击"指定xxx"时弹出多选列表
      if (val === 'spec') {
        await showFilterPicker(group);
      } else {
        // 切回"所有"时清空筛选
        if (group === 'monster') filterMonsters = [];
        if (group === 'item') filterItems = [];
        if (group === 'map') filterMaps = [];
        updateFilterBadges();
      }
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
        monsters: filterMonsters,
        items: filterItems,
        maps: filterMaps
      };
      simResult = await window.go.app.App.RunSimulation(req);
      renderSimResult(simResult);
      setStatus(`模拟完成 — 击杀:${simResult.totalKills} 掉落:${simResult.totalDrops} 空爆率:${(simResult.emptyRate*100).toFixed(1)}%`);
      addLog(`模拟完成: ${simResult.totalKills}击杀 ${simResult.totalDrops}掉落`);
    } catch(e) { setStatus('模拟失败: ' + e); }
  };

  $('btnExport').onclick = async () => {
    const text = await window.go.app.App.ExportSimResult();
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
  // 地图表（可点击）
  renderMapCol('mapTableBody', r.mapStats);
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

function renderMapCol(bodyId, mapStats) {
  const body = $(bodyId);
  let html = `<div class="rrow hdr"><span class="lbl">地图名称</span><span class="val">掉落数量</span></div>`;
  mapStats.forEach(s => {
    html += `<div class="rrow clickable" data-map="${s.mapName}"><span class="lbl">${s.mapName}</span><span class="val">${fmtNum(s.dropCount)}</span><span class="map-detail" id="mapDetail_${CSS.escape(s.mapName)}"></span></div>`;
  });
  body.innerHTML = html;
  // 绑定点击事件
  body.querySelectorAll('.rrow.clickable').forEach(row => {
    row.addEventListener('click', async () => {
      const mapName = row.dataset.map;
      const detailEl = row.querySelector('.map-detail');
      if (detailEl.innerHTML) {
        detailEl.innerHTML = '';
        return;
      }
      try {
        const monsters = await window.go.app.App.GetMonstersOnMap(mapName);
        if (monsters && monsters.length > 0) {
          detailEl.innerHTML = monsters.map(m => `<span class="map-mon-tag">${m}</span>`).join('');
        } else {
          detailEl.innerHTML = '<span style="color:var(--t3);font-size:11px">无刷怪数据</span>';
        }
      } catch(e) { detailEl.innerHTML = '<span style="color:var(--danger);font-size:11px">查询失败</span>'; }
    });
  });
}

function filterTable(bodyId, query) {
  const rows = $(bodyId).querySelectorAll('.rrow:not(.hdr)');
  const q = query.toLowerCase();
  rows.forEach(r => {
    const text = r.textContent.toLowerCase();
    r.style.display = text.includes(q) ? '' : 'none';
  });
}

// ===== 筛选选择器 =====
async function showFilterPicker(group) {
  let names = [];
  let title = '';
  let selected = [];
  try {
    if (group === 'monster') {
      names = await window.go.app.App.GetAllMonsterNames();
      title = '选择怪物';
      selected = [...filterMonsters];
    } else if (group === 'item') {
      names = await window.go.app.App.GetAllItemNames();
      title = '选择物品';
      selected = [...filterItems];
    } else if (group === 'map') {
      // 地图列表从 MonGen 推断，使用后端已有数据
      // 先尝试从模拟结果获取，若无则从加载结果获取
      names = await window.go.app.App.GetAllMapNames();
      title = '选择地图';
      selected = [...filterMaps];
    }
  } catch(e) {
    setStatus('获取列表失败: ' + e);
    return;
  }

  if (!names || names.length === 0) {
    setStatus('请先加载爆率文件');
    // 切回"所有"
    $$(`.radio[data-group="${group}"]`).forEach(r => r.classList.remove('selected'));
    $$(`.radio[data-group="${group}"][data-val="all"]`)[0].classList.add('selected');
    return;
  }

  // 构建多选列表
  const selectedSet = new Set(selected);
  const listHtml = names.map((n, i) => {
    const checked = selectedSet.has(n) ? 'checked' : '';
    return `<label style="display:flex;align-items:center;gap:6px;padding:3px 0;cursor:pointer;font-size:12px"><input type="checkbox" class="filter-cb" data-name="${n}" ${checked} /> <span>${n}</span></label>`;
  }).join('');

  const searchHtml = `<input type="text" id="filterSearch" placeholder="搜索..." style="width:100%;margin-bottom:8px;background:var(--bg0);border:1px solid var(--border);color:var(--t1);padding:4px 8px;border-radius:var(--r);font-size:12px" />`;

  showModal(title, `
    ${searchHtml}
    <div style="max-height:400px;overflow-y:auto" id="filterList">${listHtml}</div>
    <div style="margin-top:8px;display:flex;gap:8px">
      <button class="btn btn-sm" id="btnSelectAll">全选</button>
      <button class="btn btn-sm" id="btnDeselectAll">全不选</button>
    </div>
  `, [
    {text:'确定', cls:'btn-gold', action: () => {
      const checked = document.querySelectorAll('.filter-cb:checked');
      const picked = Array.from(checked).map(cb => cb.dataset.name);
      if (group === 'monster') filterMonsters = picked;
      if (group === 'item') filterItems = picked;
      if (group === 'map') filterMaps = picked;
      updateFilterBadges();
      hideModal();
    }},
    {text:'取消', cls:'', action: () => {
      hideModal();
    }}
  ]);

  // 搜索过滤
  $('filterSearch').oninput = (e) => {
    const q = e.target.value.toLowerCase();
    document.querySelectorAll('.filter-cb').forEach(cb => {
      const label = cb.closest('label');
      label.style.display = cb.dataset.name.toLowerCase().includes(q) ? '' : 'none';
    });
  };

  // 全选 / 全不选
  $('btnSelectAll').onclick = () => {
    document.querySelectorAll('.filter-cb:not([style*="display: none"])').forEach(cb => cb.checked = true);
  };
  $('btnDeselectAll').onclick = () => {
    document.querySelectorAll('.filter-cb').forEach(cb => cb.checked = false);
  };
}

function updateFilterBadges() {
  $('monsterCount').textContent = `(${filterMonsters.length})`;
  $('itemCount').textContent = `(${filterItems.length})`;
  $('mapCount').textContent = `(${filterMaps.length})`;
}

// ===== 爆率调配 =====
let tuneAnalysis = null;
let tuneRecommend = null;
let tuneUsingSim = false; // 是否使用了模拟数据

function initTuneTab() {
  // 可搜索物品选择器
  const searchInput = $('tuneItemSearch');
  const selectEl = $('tuneItemSelect');

  searchInput.addEventListener('focus', () => {
    filterTuneItems();
    selectEl.classList.add('show');
  });
  searchInput.addEventListener('input', () => {
    filterTuneItems();
    selectEl.classList.add('show');
  });
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.tune-search-wrap')) selectEl.classList.remove('show');
  });
  selectEl.addEventListener('click', (e) => {
    if (e.target.tagName === 'OPTION' && e.target.value) {
      searchInput.value = e.target.textContent;
      selectEl.value = e.target.value;
      selectEl.classList.remove('show');
    }
  });

  // 分析按钮
  $('btnAnalyze').onclick = async () => {
    const itemName = selectEl.value || searchInput.value.trim();
    if (!itemName) return setStatus('请选择或输入物品名称');
    try {
      tuneAnalysis = await window.go.app.App.AnalyzeItemDrops(itemName);
      tuneRecommend = null;
      // 判断数据来源
      tuneUsingSim = simResult != null;
      renderTuneSourceNote();
      renderTuneSources();
      renderTuneTargets();
      $('tuneRecommendSection').style.display = 'none';
      $('tuneCopySection').style.display = 'none';
      setStatus(`已分析「${itemName}」: ${tuneAnalysis.sources.length} 个掉落来源，综合期望 ${tuneAnalysis.totalExpectH.toFixed(1)} 小时/个`);
    } catch(e) { setStatus('分析失败: ' + e); }
  };

  // 计算推荐
  $('btnCalcRecommend').onclick = async () => {
    if (!tuneAnalysis) return setStatus('请先分析掉落来源');
    const itemName = tuneAnalysis.itemName;
    const targets = [];
    document.querySelectorAll('.tune-target-card').forEach(card => {
      const mapName = card.dataset.map;
      const hours = parseFloat(card.querySelector('input').value) || 0;
      if (hours > 0) targets.push({ mapName, targetHours: hours });
    });
    if (targets.length === 0) return setStatus('请至少设置一个地图的目标时间');
    try {
      tuneRecommend = await window.go.app.App.RecommendRates(itemName, targets);
      renderTuneRecommend();
      $('tuneRecommendSection').style.display = '';
      $('tuneCopySection').style.display = '';
      await fillCopyTargets(itemName);
      setStatus(`已生成 ${tuneRecommend.length} 条推荐修改`);
    } catch(e) { setStatus('计算失败: ' + e); }
  };

  // 备份
  $('btnBackupAll2').onclick = async () => {
    try {
      const path = await window.go.app.App.BackupAll();
      setStatus('已备份目录: ' + path);
      addLog('备份目录: ' + path);
    } catch(e) { setStatus('备份失败: ' + e); }
  };

  // 应用推荐值
  $('btnApplyRecommend').onclick = async () => {
    if (!tuneRecommend) return setStatus('请先计算推荐爆率');
    const rows = document.querySelectorAll('#tuneRecommendTable .rec-row');
    rows.forEach((row, i) => {
      if (i < tuneRecommend.length) {
        const denInput = row.querySelector('.rec-den');
        if (denInput) tuneRecommend[i].newDen = parseInt(denInput.value) || tuneRecommend[i].newDen;
      }
    });
    try {
      const count = await window.go.app.App.ApplyRecommendedRates(tuneAnalysis.itemName, tuneRecommend);
      setStatus(`已修改 ${count} 条爆率配置`);
      addLog(`应用推荐爆率「${tuneAnalysis.itemName}」: ${count}条`);
    } catch(e) { setStatus('应用失败: ' + e); }
  };

  // 复制爆率
  $('btnCopyRates').onclick = async () => {
    if (!tuneAnalysis) return setStatus('请先分析掉落来源');
    const sourceItem = tuneAnalysis.itemName;
    const select = $('tuneCopyTargets');
    const selected = Array.from(select.selectedOptions).map(o => o.value).filter(v => v);
    if (selected.length === 0) return setStatus('请选择目标物品');
    try {
      const result = await window.go.app.App.CopyRatesToItems(sourceItem, selected);
      setStatus(result.message);
      addLog(result.message);
    } catch(e) { setStatus('复制失败: ' + e); }
  };
}

function filterTuneItems() {
  const q = $('tuneItemSearch').value.toLowerCase();
  const options = $('tuneItemSelect').querySelectorAll('option');
  options.forEach(opt => {
    if (!opt.value) return;
    opt.style.display = opt.textContent.toLowerCase().includes(q) ? '' : 'none';
  });
}

function renderTuneSourceNote() {
  const note = $('tuneSourceNote');
  if (tuneUsingSim) {
    note.innerHTML = '💡 数据来源：<b style="color:var(--success)">模拟爆率</b> — 基于最近一次模拟结果分析，数据更贴近实际游戏表现';
    note.style.borderLeftColor = 'var(--success)';
  } else {
    note.innerHTML = '⚠️ 数据来源：<b style="color:var(--accent)">爆率文件 + 刷怪文件</b> — 理论计算值，建议先运行「爆率模拟」获取更准确的数据';
    note.style.borderLeftColor = 'var(--accent)';
  }
}

function renderTuneSources() {
  if (!tuneAnalysis) return;
  $('tuneSourceCount').textContent = `(${tuneAnalysis.sources.length}个来源)`;
  let html = `<table class="tune-table">
    <tr><th class="col-map">地图</th><th class="col-mon">怪物</th><th class="col-prob">爆率</th><th class="col-qty">数量</th><th class="col-kph">每小时怪数</th><th class="col-time">期望(小时/个)</th></tr>`;
  tuneAnalysis.sources.forEach(s => {
    const timeCls = s.expectHours <= 2 ? 'time-good' : s.expectHours <= 10 ? 'time-warn' : 'time-bad';
    html += `<tr>
      <td class="col-map" title="${s.mapName}">${s.mapName}</td>
      <td class="col-mon" title="${s.monsterName}">${s.monsterName}</td>
      <td class="col-prob num prob">${s.probStr}</td>
      <td class="col-qty num">${s.quantity}</td>
      <td class="col-kph num">${s.killPerHour.toFixed(1)}</td>
      <td class="col-time num ${timeCls}">${s.expectHours.toFixed(1)}</td>
    </tr>`;
  });
  html += `<tr style="font-weight:600;background:var(--bg2)"><td class="col-map" colspan="2">综合</td><td class="col-prob"></td><td class="col-qty"></td><td class="col-kph num">${tuneAnalysis.totalKph.toFixed(1)}</td><td class="col-time num">${tuneAnalysis.totalExpectH.toFixed(1)}</td></tr>`;
  html += '</table>';
  $('tuneSourceTable').innerHTML = html;
}

function renderTuneTargets() {
  if (!tuneAnalysis) return;
  const maps = [];
  const seen = new Set();
  tuneAnalysis.sources.forEach(s => {
    if (!seen.has(s.mapName)) {
      seen.add(s.mapName);
      maps.push({ mapName: s.mapName, currentH: s.expectHours });
    }
  });
  let html = '';
  maps.forEach(m => {
    const defaultVal = Math.max(1, Math.round(m.currentH));
    html += `<div class="tune-target-card" data-map="${m.mapName}">
      <label>${m.mapName}:</label>
      <input type="text" value="${defaultVal}" /> 小时/个
      <span style="color:var(--t3);font-size:11px">(当前≈${m.currentH.toFixed(1)}h)</span>
    </div>`;
  });
  $('tuneTargets').innerHTML = html;
}

function renderTuneRecommend() {
  if (!tuneRecommend) return;
  $('tuneRecommendCount').textContent = `(${tuneRecommend.length}条)`;
  let html = `<table class="tune-table">
    <tr><th class="col-map">地图</th><th class="col-mon">怪物</th><th class="col-prob">当前爆率</th><th class="col-qty">推荐爆率</th><th class="col-time">修改后期望</th></tr>`;
  tuneRecommend.forEach((c) => {
    const oldProb = c.oldNum + '/' + c.oldDen;
    html += `<tr class="rec-row">
      <td class="col-map" title="${c.mapName}">${c.mapName}</td>
      <td class="col-mon" title="${c.monsterName}">${c.monsterName}</td>
      <td class="col-prob num prob">${oldProb}</td>
      <td class="col-qty num">${c.newNum}/<input type="text" class="rec-den" value="${c.newDen}" style="width:65px" /></td>
      <td class="col-time num">${c.newExpectH.toFixed(1)}h</td>
    </tr>`;
  });
  html += '</table>';
  $('tuneRecommendTable').innerHTML = html;
}

async function fillCopyTargets(excludeItem) {
  try {
    const allItems = await window.go.app.App.GetAllItemNames();
    const select = $('tuneCopyTargets');
    select.innerHTML = '';
    (allItems || []).filter(n => n !== excludeItem).forEach(name => {
      const opt = document.createElement('option');
      opt.value = name;
      opt.textContent = name;
      select.appendChild(opt);
    });
  } catch(e) { console.log('fillCopyTargets:', e); }
}

async function refreshTuneItemList() {
  try {
    const allItems = await window.go.app.App.GetAllItemNames();
    const select = $('tuneItemSelect');
    const search = $('tuneItemSearch');
    select.innerHTML = '<option value="">-- 请选择物品 --</option>';
    search.value = '';
    (allItems || []).forEach(name => {
      const opt = document.createElement('option');
      opt.value = name;
      opt.textContent = name;
      select.appendChild(opt);
    });
  } catch(e) { console.log('refreshTuneItemList:', e); }
}

// ===== 授权 =====
function initAuthTab() {
  $('btnCopyCode').onclick = async () => {
    const code = await window.go.app.App.GetMachineID();
    $('machineCode').textContent = code;
    navigator.clipboard.writeText(code);
    setStatus('机器码已复制');
  };

  $('btnActivate').onclick = async () => {
    const code = $('activateCode').value.trim();
    if (!code) return setStatus('请输入激活码');
    try {
      const info = await window.go.app.App.Activate(code);
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
    const info = await window.go.app.App.GetLicenseStatus();
    if (info.isActive) {
      $('licenseStatus').textContent = '✅ 已激活 (' + info.type + ')';
      $('licenseStatus').className = 'status-badge active';
    } else {
      $('licenseStatus').textContent = '未激活';
      $('licenseStatus').className = 'status-badge inactive';
    }
    const code = await window.go.app.App.GetMachineID();
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