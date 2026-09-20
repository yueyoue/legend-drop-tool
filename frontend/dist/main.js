// ===== Wails 后端绑定 =====
// Wails v2 自动生成 window.go.app.App.* 方法

// ===== 状态 =====
let currentMonsterIdx = -1;
let monsters = [];
let simResult = null;
let logEntries = [];
let selectedSimItem = ''; // 当前模拟结果中选中的物品名（用于级联筛选）
let mapViewMode = false; // 地图视图模式
let mapViewData = null;   // 地图视图缓存数据

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
    document.getElementById('setupStatus').textContent = '错误: ' + e.message + ' — 请确认使用最新版 exe';
    return;
  }
  initSetupPage();
});

// 启动设置页
function initSetupPage() {
  const dbTypeSelect = $('setupDBType');
  // 数据库类型切换
  dbTypeSelect.onchange = () => {
    const t = dbTypeSelect.value;
    const isFile = ['BDE','Access','SQLite','Excel'].includes(t);
    $('setupFileRow').style.display = isFile ? '' : 'none';
    $('setupConnRow').style.display = isFile ? 'none' : '';
    // 默认端口
    if (t === 'MYSQL') $('setupPort').value = '3306';
    if (t === 'SQL Server') $('setupPort').value = '1433';
    // 默认路径提示
    const pathHints = {
      'BDE': '\\MirServer\\DBServer\\FilId.DB',
      'Access': '\\MirServer\\DBServer\\Mir.DB.MDB',
      'SQLite': '\\MirServer\\DBServer\\Mir.DB.db3',
      'Excel': '\\Mir200\\Envir\\Data\\cfg_equip.xls'
    };
    $('setupDBPath').placeholder = pathHints[t] || '数据库文件路径';
  };
  dbTypeSelect.onchange();

  // 浏览按钮
  $('setupBrowse').onclick = async () => {
    try {
      const result = await window.go.app.App.SelectDirectory();
      if (result) {
        $('setupServerPath').value = result;
        // 自动填充数据库路径
        const t = dbTypeSelect.value;
        const autoPaths = {
          'BDE': '\\DBServer\\FilId.DB',
          'Access': '\\DBServer\\Mir.DB.MDB',
          'SQLite': '\\DBServer\\Mir.DB.db3',
          'Excel': '\\Mir200\\Envir\\Data\\cfg_equip.xls'
        };
        if (autoPaths[t]) {
          $('setupDBPath').value = result + autoPaths[t];
        }
      }
    } catch(e) {}
  };
  $('setupDBBrowse').onclick = async () => {
    try {
      const t = dbTypeSelect.value;
      const titles = {
        'BDE': '选择 BDE 数据库文件 (*.DB)|*.DB',
        'Access': '选择 Access 数据库文件 (*.MDB)|*.MDB',
        'SQLite': '选择 SQLite 数据库文件 (*.db3;*.db)|*.db3;*.db',
        'Excel': '选择 Excel 文件 (*.xls;*.xlsx)|*.xls;*.xlsx'
      };
      const title = titles[t] || '选择数据库文件';
      const result = await window.go.app.App.SelectFile(title);
      if (result) $('setupDBPath').value = result;
    } catch(e) {
      $('setupStatus').textContent = '选择文件失败: ' + e;
    }
  };

  // 开始使用
  $('btnStartApp').onclick = async () => {
    const serverPath = $('setupServerPath').value;
    if (!serverPath) { $('setupStatus').textContent = '请选择服务端目录'; return; }

    const dbType = dbTypeSelect.value;
    const isFile = ['BDE','Access','SQLite','Excel'].includes(dbType);

    // 设置服务端路径
    await window.go.app.App.SetServerPath(serverPath);

    // 加载爆率文件
    $('setupStatus').textContent = '正在加载爆率文件...';
    try {
      const result = await window.go.app.App.LoadFiles(serverPath);
      monsters = result.monsters;
    } catch(e) {
      $('setupStatus').textContent = '加载爆率文件失败: ' + e;
      return;
    }

    // 连接数据库
    $('setupStatus').textContent = '正在连接数据库...';
    try {
      const dbCfg = {
        type: dbType,
        filePath: isFile ? $('setupDBPath').value : '',
        host: $('setupHost').value,
        port: parseInt($('setupPort').value) || 0,
        dbName: $('setupDBName').value,
        user: $('setupUser').value,
        password: $('setupPassword').value
      };
      const count = await window.go.app.App.ConnectDB(dbCfg);
      $('setupStatus').textContent = `已读取 ${count} 个物品`;
    } catch(e) {
      $('setupStatus').textContent = '数据库连接失败: ' + e;
      return;
    }

    // 设置引擎
    const engine = $('setupEngine').value;
    if (engine !== '自动检测') {
      await window.go.app.App.SetEngine(engine);
    }

    // 进入主界面
    $('setupPage').style.display = 'none';
    $('mainApp').style.display = '';
    initMainApp();
  };
}

function initMainApp() {
  initTabs();
  initToolbar();
  initEditTab();
  initMapViewSearch();
  initItemPanelSearch();
  initCtxMenu();
  initSimTab();
  initTuneTab();
  initAuthTab();
  renderMonsterList();
  refreshTuneItemList();
  loadItemPanel();
  $('editTopnav').style.display = '';
  setStatus(`已加载 ${monsters.length} 个怪物文件`);
}

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
      const isEdit = tab.dataset.tab === 'edit';
      $('sidebar').style.display = isEdit ? 'flex' : 'none';
      $('editTopnav').style.display = isEdit ? '' : 'none';
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

  // 引擎切换
  $('engineSelect').onchange = () => {
    window.go.app.App.SetEngine($('engineSelect').value);
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
      showLoading('正在加载爆率文件，请稍候...');
      const result = await window.go.app.App.LoadFiles(path);
      monsters = result.monsters;
      renderMonsterList();
      refreshTuneItemList();
      setStatus(`已加载 ${result.totalFiles} 个怪物文件，共 ${result.totalEntries} 条掉落配置 | 引擎: ${result.engine}`);
      if (result.warnings && result.warnings.length > 0) {
        showModal('解析提示', `<div style="max-height:400px;overflow:auto;font-family:monospace;font-size:12px;white-space:pre">${result.warnings.join('\n')}</div>`, [{text:'确定',cls:'btn-gold',action:hideModal}]);
      }
    } catch(e) { setStatus('错误: ' + e); }
    finally { hideLoading(); }
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
  // 同步更新物品面板
  if (dbItems.length > 0) renderItemPanel(idx);
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
  // 地图视图/怪物列表切换（顶部导航栏按钮）
  const btnVM = $('btnViewMonster');
  const btnVM2 = document.getElementById('btnViewMonster2');
  const btnVMap = $('btnViewMap');
  const btnVMap2 = document.getElementById('btnViewMap2');

  function switchToMonsterView() {
    if (!mapViewMode) return;
    mapViewMode = false;
    document.querySelectorAll('.view-toggle').forEach(b => b.classList.remove('active'));
    if (btnVM) btnVM.classList.add('active');
    if (btnVM2) btnVM2.classList.add('active');
    $('sidebar').style.display = 'flex';
    $('editMainArea').style.display = '';
    $('mapViewContainer').style.display = 'none';
  }
  function switchToMapView() {
    if (mapViewMode) return;
    mapViewMode = true;
    document.querySelectorAll('.view-toggle').forEach(b => b.classList.remove('active'));
    if (btnVMap) btnVMap.classList.add('active');
    if (btnVMap2) btnVMap2.classList.add('active');
    $('sidebar').style.display = 'none';
    $('editMainArea').style.display = 'none';
    $('mapViewContainer').style.display = 'flex';
    loadMapView();
  }

  if (btnVM) btnVM.onclick = switchToMonsterView;
  if (btnVMap) btnVMap.onclick = switchToMapView;

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
      showLoading('正在运行爆率模拟，请稍候...');
      setStatus('正在模拟...');
      const req = {
        durationHours: +$('simDuration').value,
        killRatioPct: +$('simKillRatio').value,
        playerRatePct: +$('simPlayerRate').value || 100,
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
    finally { hideLoading(); }
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
  selectedSimItem = ''; // 重置选中物品
  // 物品表（支持点击筛选）
  renderCol('itemTableBody', ['物品名称','掉落数量'], r.itemStats.map(s=>[s.itemName, fmtNum(s.dropCount)]), 'item', r);
  // 地图表（支持点击筛选）— 使用 displayName 显示
  renderCol('mapTableBody', ['地图名称','掉落数量'], r.mapStats.map(s=>[s.displayName || s.mapName, fmtNum(s.dropCount)]), 'map', r, r.mapStats.map(s=>s.mapName));
  // 怪物表（支持双击打开爆率文件）
  renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], r.monsterStats.map(s=>[s.monsterName, `${fmtNum(s.killCount)} / ${fmtNum(s.dropCount)}`]), 'monster', r);
  // 统计
  $('sumKill').textContent = '总击杀: ' + fmtNum(r.totalKills);
  $('sumDrop').textContent = '总掉落: ' + fmtNum(r.totalDrops);
  $('sumEmpty').textContent = '空爆率: ' + (r.emptyRate*100).toFixed(1) + '%';
  $('sumTypes').textContent = '物品种类: ' + r.itemStats.length;
  // 稀有追踪
  const rare = r.itemStats.filter(s => s.prob > 0 && s.prob < 0.001).slice(0, 10);
  $('trackerItems').innerHTML = rare.map(s => `<span class="tracker-item">${s.itemName} ×${fmtNum(s.dropCount)} (${(s.prob*100).toFixed(3)}%)</span>`).join('');
}

function renderCol(bodyId, headers, rows, type, simData, dataNames) {
  const body = $(bodyId);
  let html = `<div class="rrow hdr"><span class="lbl">${headers[0]}</span><span class="val">${headers[1]}</span></div>`;
  rows.forEach((r, i) => {
    const dname = (dataNames && dataNames[i]) ? dataNames[i] : r[0];
    const clickAttr = (type === 'item' || type === 'map') ? ` data-idx="${i}" data-name="${dname}" style="cursor:pointer"` : (type === 'monster') ? ` data-idx="${i}" data-name="${dname}"` : '';
    html += `<div class="rrow"${clickAttr}><span class="lbl">${r[0]}</span><span class="val">${r[1]}</span></div>`;
  });
  body.innerHTML = html;
  // 物品行点击 → 筛选地图和怪物（再次点击取消筛选）
  if (type === 'item' && simData) {
    body.querySelectorAll('.rrow[data-idx]').forEach(el => {
      el.onclick = () => {
        const itemName = el.dataset.name;
        const wasSelected = el.classList.contains('selected');
        body.querySelectorAll('.rrow').forEach(r => r.classList.remove('selected'));
        if (wasSelected) {
          // 取消选中，恢复全部数据
          selectedSimItem = '';
          renderCol('mapTableBody', ['地图名称','掉落数量'], simData.mapStats.map(s=>[s.displayName || s.mapName, fmtNum(s.dropCount)]), 'map', simData, simData.mapStats.map(s=>s.mapName));
          renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], simData.monsterStats.map(s=>[s.monsterName, `${fmtNum(s.killCount)} / ${fmtNum(s.dropCount)}`]), 'monster', simData);
          setStatus('已取消物品筛选');
        } else {
          el.classList.add('selected');
          filterByItem(itemName, simData);
        }
      };
    });
  }
  // 地图行点击 → 筛选怪物（再次点击取消筛选）
  if (type === 'map' && simData) {
    body.querySelectorAll('.rrow[data-idx]').forEach(el => {
      el.onclick = () => {
        const mapName = el.dataset.name;
        const wasSelected = el.classList.contains('selected');
        body.querySelectorAll('.rrow').forEach(r => r.classList.remove('selected'));
        if (wasSelected) {
          // 取消选中，恢复怪物列表
          if (selectedSimItem) {
            // 有物品选中时，恢复到物品筛选状态
            filterByItem(selectedSimItem, simData);
          } else {
            renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], simData.monsterStats.map(s=>[s.monsterName, `${fmtNum(s.killCount)} / ${fmtNum(s.dropCount)}`]), 'monster', simData);
          }
          setStatus('已取消地图筛选');
        } else {
          el.classList.add('selected');
          filterByMap(mapName, simData);
        }
      };
    });
  }
  // 怪物行双击 → 打开爆率文件
  if (type === 'monster') {
    body.querySelectorAll('.rrow[data-idx]').forEach(el => {
      el.ondblclick = async () => {
        const monsterName = el.dataset.name;
        const idx = monsters.findIndex(m => m.name === monsterName);
        if (idx >= 0) {
          $$('.tab').forEach(t => t.classList.remove('active'));
          $$('.tab-pane').forEach(p => p.classList.remove('active'));
          document.querySelector('.tab[data-tab="edit"]').classList.add('active');
          $('pane-edit').classList.add('active');
          $('sidebar').style.display = 'flex';
          await selectMonster(idx);
          setStatus(`已打开怪物: ${monsterName}`);
        }
      };
      el.style.cursor = 'pointer';
      el.title = '双击打开爆率文件';
    });
  }
}

function filterByItem(itemName, simData) {
  selectedSimItem = itemName; // 记录当前选中的物品
  // 筛选地图
  const mapDrops = (simData.itemMapDrops || {})[itemName] || {};
  const mapRows = Object.entries(mapDrops)
    .map(([name, cnt]) => [name, fmtNum(cnt)])
    .sort((a, b) => parseInt(b[1].replace(/,/g,'')) - parseInt(a[1].replace(/,/g,'')));
  // 用 displayName 显示，data-name 保持 mapName
  const displayNames = mapRows.map(r => {
    const ms = (simData.mapStats || []).find(s => s.mapName === r[0]);
    return ms ? (ms.displayName || ms.mapName) : r[0];
  });
  renderCol('mapTableBody', ['地图名称','掉落数量'], mapRows.map((r,i) => [displayNames[i], r[1]]), 'map', simData, mapRows.map(r=>r[0]));

  // 筛选怪物
  const monDrops = (simData.itemMonsterDrops || {})[itemName] || {};
  const monRows = [];
  for (const [monName, dropCnt] of Object.entries(monDrops)) {
    const ms = (simData.monsterStats || []).find(m => m.monsterName === monName);
    const killCnt = ms ? ms.killCount : 0;
    monRows.push([monName, `${fmtNum(killCnt)} / ${fmtNum(dropCnt)}`]);
  }
  monRows.sort((a, b) => {
    const da = parseInt(a[1].split('/')[1].trim().replace(/,/g,''));
    const db = parseInt(b[1].split('/')[1].trim().replace(/,/g,''));
    return db - da;
  });
  renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], monRows, 'monster', simData);

  setStatus(`已选中物品: ${itemName} | 地图:${mapRows.length}个 怪物:${monRows.length}个`);
}

function filterByMap(mapName, simData) {
  const monRows = [];

  if (selectedSimItem && (simData.itemMonsterMapDrops || {})[selectedSimItem]) {
    // 级联模式：已选中物品，筛选该物品在该地图的怪物掉落
    const monsterMapDrops = simData.itemMonsterMapDrops[selectedSimItem];
    for (const [monName, mapCnts] of Object.entries(monsterMapDrops)) {
      const cnt = mapCnts[mapName] || 0;
      if (cnt > 0) {
        const ms = (simData.monsterStats || []).find(m => m.monsterName === monName);
        const killCnt = ms ? ms.killCount : 0;
        monRows.push([monName, `${fmtNum(killCnt)} / ${fmtNum(cnt)}`]);
      }
    }
    monRows.sort((a, b) => {
      const da = parseInt(a[1].split('/')[1].trim().replace(/,/g,''));
      const db = parseInt(b[1].split('/')[1].trim().replace(/,/g,''));
      return db - da;
    });
    renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], monRows, 'monster', simData);
    setStatus(`已选中物品: ${selectedSimItem} + 地图: ${mapName} | 怪物:${monRows.length}个`);
  } else {
    // 全局模式：未选中物品，显示该地图所有怪物
    const monMap = {};
    const imm = simData.itemMonsterMapDrops || {};
    for (const [itemName, monsterMaps] of Object.entries(imm)) {
      for (const [monName, mapCnts] of Object.entries(monsterMaps)) {
        const cnt = mapCnts[mapName] || 0;
        if (cnt > 0) {
          if (!monMap[monName]) monMap[monName] = { drops: 0 };
          monMap[monName].drops += cnt;
        }
      }
    }
    for (const [name, info] of Object.entries(monMap)) {
      const ms = (simData.monsterStats || []).find(m => m.monsterName === name);
      const killCnt = ms ? ms.killCount : 0;
      monRows.push([name, `${fmtNum(killCnt)} / ${fmtNum(info.drops)}`]);
    }
    monRows.sort((a, b) => {
      const da = parseInt(a[1].split('/')[1].trim().replace(/,/g,''));
      const db = parseInt(b[1].split('/')[1].trim().replace(/,/g,''));
      return db - da;
    });
    renderCol('monTableBody', ['怪物名称','击杀 / 掉落'], monRows, 'monster', simData);
    setStatus(`已选中地图: ${mapName} | 怪物:${monRows.length}个`);
  }
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

// ===== 物品面板（数据库物品列表） =====
let dbItems = []; // 缓存数据库物品
let itemPanelMonsterIdx = -1; // 当前物品面板关联的怪物索引

async function loadItemPanel() {
  try {
    dbItems = await window.go.app.App.GetDBItems() || [];
    $('itemPanel').style.display = '';
    renderItemPanel(-1);
  } catch(e) {
    console.log('loadItemPanel:', e);
  }
}

async function renderItemPanel(monsterIdx) {
  itemPanelMonsterIdx = monsterIdx;
  const body = $('itemPanelBody');
  const configured = {};

  if (monsterIdx >= 0) {
    try {
      const cfg = await window.go.app.App.CheckItemConfigured(monsterIdx);
      Object.assign(configured, cfg);
    } catch(e) {}
  }

  const query = ($('itemPanelSearch').value || '').toLowerCase();
  let html = '';
  let count = 0;
  dbItems.forEach(item => {
    if (query && !item.Name.toLowerCase().includes(query)) return;
    const isConfigured = configured[item.Name] === true;
    const cls = isConfigured ? 'configured' : 'unconfigured';
    const check = isConfigured ? '✓' : '';
    html += `<div class="ip-item ${cls}" data-name="${item.Name}"><span class="ip-check">${check}</span><span class="ip-name" title="${item.Name}">${item.Name}</span></div>`;
    count++;
  });
  body.innerHTML = html;
  $('itemPanelCount').textContent = `(${count})`;

  // 点击物品 → 快速添加爆率
  body.querySelectorAll('.ip-item').forEach(el => {
    el.onclick = async () => {
      const itemName = el.dataset.name;
      if (itemPanelMonsterIdx < 0) return setStatus('请先选择怪物');
      const monName = monsters[itemPanelMonsterIdx]?.name || '';
      showQuickAddDialog(itemPanelMonsterIdx, itemName, monName);
    };
  });
}

function showQuickAddDialog(monsterIdx, itemName, monsterName) {
  showModal('快速添加爆率', `
    <div class="form-row"><label>怪物:</label><span style="color:#5b9df0">${monsterName}</span></div>
    <div class="form-row"><label>物品:</label><span style="color:#4ecb71">${itemName}</span></div>
    <div class="form-row"><label>概率分子:</label><input type="text" id="mNum" value="1" /></div>
    <div class="form-row"><label>概率分母:</label><input type="text" id="mDen" value="100" /></div>
    <div class="form-row"><label>掉落数量:</label><input type="text" id="mQty" value="1" /></div>
  `, [
    {text:'添加',cls:'btn-gold',action:async()=>{
      const num=+$('mNum').value,den=+$('mDen').value,qty=+$('mQty').value;
      if(den<=0||qty<=0) return setStatus('请输入有效正整数');
      await window.go.app.App.QuickAddItem(monsterIdx, itemName, num, den, qty);
      hideModal();
      loadEntries(monsterIdx);
      renderItemPanel(monsterIdx);
      addLog(`快速添加: ${itemName} → ${monsters[monsterIdx]?.name} ${num}/${den} x${qty}`);
    }},
    {text:'取消',cls:'',action:hideModal}
  ]);
}

// 物品面板搜索
function initItemPanelSearch() {
  $('itemPanelSearch').oninput = () => renderItemPanel(itemPanelMonsterIdx);
}

// ===== 地图视图（按地图查看怪物和爆率） =====
async function loadMapView() {
  try {
    showLoading('正在加载地图视图...');
    const maps = await window.go.app.App.GetMapMonsterView();
    // 预加载所有地图的怪物，构建怪物→出现地图映射
    const monsterMapLocations = {}; // monsterName → [displayName, ...]
    for (const m of (maps || [])) {
      try {
        const mons = await window.go.app.App.GetMapMonsters(m.mapName);
        (mons || []).forEach(mon => {
          if (!monsterMapLocations[mon.monsterName]) monsterMapLocations[mon.monsterName] = [];
          monsterMapLocations[mon.monsterName].push(m.displayName);
        });
      } catch(e) { /* skip */ }
    }
    mapViewData = { maps: maps || [], selectedMap: '', selectedMonster: -1, monsterMapLocations };
    renderMapViewMaps();
    $('mapViewMonBody').innerHTML = '';
    $('mapViewEntryBody').innerHTML = '';
    $('mapViewMapCount').textContent = `(${(maps||[]).length}个)`;
    $('mapViewMonCount').textContent = '';
    $('mapViewItemCount').textContent = '';
  } catch(e) {
    setStatus('加载地图视图失败: ' + e);
  } finally {
    hideLoading();
  }
}

function renderMapViewMaps() {
  const body = $('mapViewMapBody');
  const maps = (mapViewData && mapViewData.maps) || [];
  let html = `<div class="rrow hdr"><span class="lbl">地图名称</span><span class="val">怪物数</span></div>`;
  maps.forEach((m, i) => {
    html += `<div class="rrow" data-idx="${i}" data-name="${m.mapName}"><span class="lbl">${m.displayName}</span><span class="val">${m.monsterCount}</span></div>`;
  });
  body.innerHTML = html;
  // 点击地图 → 加载怪物列表
  body.querySelectorAll('.rrow[data-idx]').forEach(el => {
    el.onclick = async () => {
      body.querySelectorAll('.rrow').forEach(r => r.classList.remove('selected'));
      el.classList.add('selected');
      const mapName = el.dataset.name;
      mapViewData.selectedMap = mapName;
      mapViewData.selectedMonster = -1;
      await loadMapViewMonsters(mapName);
    };
  });
}

// 复制爆率缓存
let copiedMonsterRates = null; // { monsterIndex, entries }

async function loadMapViewMonsters(mapName) {
  try {
    const monsters = await window.go.app.App.GetMapMonsters(mapName);
    const body = $('mapViewMonBody');
    const list = monsters || [];
    // 从缓存获取多地图怪物信息
    const locMap = (mapViewData && mapViewData.monsterMapLocations) || {};
    // 获取当前地图显示名
    const curMapDisplay = ((mapViewData.maps)||[]).find(m=>m.mapName===mapName);
    const curDisplayName = curMapDisplay ? curMapDisplay.displayName : mapName;

    let html = `<div class="rrow hdr"><span class="lbl">怪物名称</span><span class="val">掉落条目</span></div>`;
    list.forEach((m, i) => {
      const allMaps = locMap[m.monsterName] || [];
      // 过滤掉当前地图
      const otherMaps = allMaps.filter(d => d !== curDisplayName);
      const multiCls = otherMaps.length > 0 ? ' multi-map' : '';
      const tooltip = otherMaps.length > 0 ? ` title="该怪物还出现在: ${otherMaps.join(', ')}"` : '';
      html += `<div class="rrow${multiCls}" data-idx="${i}" data-name="${m.monsterName}" data-midx="${m.monsterIndex}"${tooltip}>
        <span class="lbl"><label class="mon-cb-wrap"><input type="checkbox" class="mon-cb" data-midx="${m.monsterIndex}" /> ${m.monsterName}</label></span>
        <span class="val">${m.entryCount}条</span>
      </div>`;
    });
    body.innerHTML = html;
    $('mapViewMonCount').textContent = `(${list.length}个)`;
    $('mapViewEntryBody').innerHTML = '';
    $('mapViewItemCount').textContent = '';
    // 点击怪物行 → 加载掉落条目（点击 checkbox 区域不触发）
    body.querySelectorAll('.rrow[data-idx]').forEach(el => {
      el.onclick = async (e) => {
        if (e.target.classList.contains('mon-cb')) return; // 点击 checkbox 不触发
        body.querySelectorAll('.rrow').forEach(r => r.classList.remove('selected'));
        el.classList.add('selected');
        const monsterIdx = parseInt(el.dataset.midx);
        mapViewData.selectedMonster = monsterIdx;
        await loadMapViewEntries(monsterIdx);
      };
      // 右键菜单
      el.oncontextmenu = (e) => {
        e.preventDefault();
        showCtxMenu(e.pageX, e.pageY, parseInt(el.dataset.midx));
      };
    });
  } catch(e) {
    setStatus('加载怪物列表失败: ' + e);
  }
}

// 右键菜单
function showCtxMenu(x, y, monsterIdx) {
  const menu = $('ctxMenu');
  menu.style.left = x + 'px';
  menu.style.top = y + 'px';
  menu.classList.add('show');
  menu.dataset.midx = monsterIdx;
}
function hideCtxMenu() { $('ctxMenu').classList.remove('show'); }

document.addEventListener('click', (e) => {
  if (!e.target.closest('.ctx-menu')) hideCtxMenu();
});

function initCtxMenu() {
  // 复制爆率
  $('ctxCopyRates').onclick = async () => {
    const idx = parseInt($('ctxMenu').dataset.midx);
    hideCtxMenu();
    try {
      const entries = await window.go.app.App.GetEntries(idx);
      copiedMonsterRates = { monsterIndex: idx, entries: entries };
      const name = monsters[idx]?.name || '';
      setStatus(`已复制「${name}」的爆率配置 (${(entries||[]).filter(e=>e.isEditable).length}条)`);
    } catch(e) { setStatus('复制失败: ' + e); }
  };
  // 粘贴爆率
  $('ctxPasteRates').onclick = async () => {
    hideCtxMenu();
    if (!copiedMonsterRates) return setStatus('请先复制爆率');
    const checked = document.querySelectorAll('#mapViewMonBody .mon-cb:checked');
    const targets = Array.from(checked).map(cb => parseInt(cb.dataset.midx)).filter(i => i !== copiedMonsterRates.monsterIndex);
    if (targets.length === 0) return setStatus('请先勾选目标怪物（checkbox）');
    try {
      showLoading('正在粘贴爆率...');
      const count = await window.go.app.App.CopyMonsterRates(copiedMonsterRates.monsterIndex, targets);
      const srcName = monsters[copiedMonsterRates.monsterIndex]?.name || '';
      setStatus(`已将「${srcName}」的爆率复制到 ${count} 个怪物`);
      addLog(`复制爆率: ${srcName} → ${count}个怪物`);
      // 刷新当前地图怪物列表
      if (mapViewData.selectedMap) await loadMapViewMonsters(mapViewData.selectedMap);
    } catch(e) { setStatus('粘贴失败: ' + e); }
    finally { hideLoading(); }
  };
  // 全选/全不选
  $('ctxSelectAll').onclick = () => {
    hideCtxMenu();
    document.querySelectorAll('#mapViewMonBody .mon-cb').forEach(cb => cb.checked = true);
  };
  $('ctxDeselectAll').onclick = () => {
    hideCtxMenu();
    document.querySelectorAll('#mapViewMonBody .mon-cb').forEach(cb => cb.checked = false);
  };
}

async function loadMapViewEntries(monsterIndex) {
  try {
    const entries = await window.go.app.App.GetEntries(monsterIndex);
    renderMapViewEntries(entries, monsterIndex);
  } catch(e) {
    setStatus('加载掉落条目失败: ' + e);
  }
}

function renderMapViewEntries(entries, monsterIndex) {
  const body = $('mapViewEntryBody');
  if (!entries) { body.innerHTML = ''; return; }
  let count = 0;
  let html = '';
  entries.forEach((e, i) => {
    if (e.isComment && !e.isEditable) return; // 跳过纯注释行
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
      html += `<div class="entry" data-eidx="${e.index}"><span class="icon">${icon}</span><span>${indent}${text}</span></div>`;
      count++;
      return;
    }
    else { text = e.rawLine; }
    html += `<div class="entry"><span class="icon ${cls}">${icon}</span><span class="${cls}">${indent}${text}</span></div>`;
  });
  body.innerHTML = html;
  $('mapViewItemCount').textContent = `(${count}条可编辑)`;
  // 点击可编辑条目 → 弹出编辑对话框（复用已有 showEditDialog）
  body.querySelectorAll('.entry[data-eidx]').forEach(el => {
    el.onclick = () => {
      const entryIdx = parseInt(el.dataset.eidx);
      const entry = entries.find(e => e.index === entryIdx);
      if (entry && entry.isEditable) {
        showMapViewEditDialog(monsterIndex, entryIdx, entry);
      }
    };
  });
}

function showMapViewEditDialog(monsterIndex, entryIndex, entry) {
  showModal('修改掉落配置', `
    <div class="form-row"><label>怪物:</label><span style="color:#5b9df0">${monsters.find((m,i) => i === monsterIndex)?.name || ''}</span></div>
    <div class="form-row"><label>物品名称:</label><span style="color:#5b9df0">${entry.itemName}</span></div>
    <div class="form-row"><label>概率分子:</label><input type="text" id="mNum" value="${entry.probNum}" /></div>
    <div class="form-row"><label>概率分母:</label><input type="text" id="mDen" value="${entry.probDen}" /></div>
    <div class="form-row"><label>掉落数量:</label><input type="text" id="mQty" value="${entry.quantity}" /></div>
  `, [
    {text:'保存',cls:'btn-gold',action:async()=>{
      const num=+$('mNum').value,den=+$('mDen').value,qty=+$('mQty').value;
      if(den<=0||qty<=0) return setStatus('请输入有效正整数');
      await window.go.app.App.ModifyEntry(monsterIndex,entryIndex,num,den,qty);
      hideModal();
      // 刷新条目列表
      await loadMapViewEntries(monsterIndex);
      addLog(`修改 ${entry.itemName}: ${num}/${den} x${qty}`);
    }},
    {text:'取消',cls:'',action:hideModal}
  ]);
}

// 地图视图搜索过滤
function initMapViewSearch() {
  $('mapViewMapSearch').oninput = (e) => {
    const q = e.target.value.toLowerCase();
    const rows = $('mapViewMapBody').querySelectorAll('.rrow:not(.hdr)');
    rows.forEach(r => {
      const text = r.textContent.toLowerCase();
      r.style.display = text.includes(q) ? '' : 'none';
    });
  };
  $('mapViewMonSearch').oninput = (e) => {
    const q = e.target.value.toLowerCase();
    const rows = $('mapViewMonBody').querySelectorAll('.rrow:not(.hdr)');
    rows.forEach(r => {
      const text = r.textContent.toLowerCase();
      r.style.display = text.includes(q) ? '' : 'none';
    });
  };
}

// ===== 爆率调配 =====
let tuneAnalysis = null;
let tuneRecommend = null;

function initTuneTab() {
  // 物品搜索过滤（重建 select options）
  let allTuneItems = [];
  $('tuneItemSearch').oninput = (e) => {
    const q = e.target.value.toLowerCase();
    const select = $('tuneItemSelect');
    const curVal = select.value;
    select.innerHTML = '<option value="">-- 请选择物品 --</option>';
    allTuneItems.filter(n => n.toLowerCase().includes(q)).forEach(name => {
      const opt = document.createElement('option');
      opt.value = name;
      opt.textContent = name;
      select.appendChild(opt);
    });
    // 恢复之前选中的值
    if (allTuneItems.includes(curVal) && curVal.toLowerCase().includes(q)) {
      select.value = curVal;
    }
  };
  // 保存物品列表供搜索用
  window._setAllTuneItems = (items) => { allTuneItems = items || []; };

  // 分析按钮
  $('btnAnalyze').onclick = async () => {
    const itemName = $('tuneItemSelect').value;
    if (!itemName) return setStatus('请选择物品');
    try {
      const hasSim = simResult != null;
      tuneAnalysis = await window.go.app.App.AnalyzeItemDrops(itemName);
      tuneRecommend = null;
      renderTuneSources(hasSim);
      renderTuneTargets();
      $('tuneRecommendSection').style.display = 'none';
      $('tuneCopySection').style.display = 'none';
      const src = hasSim ? '模拟数据' : '配置文件+MonGen数据';
      setStatus(`已分析「${itemName}」: ${tuneAnalysis.sources.length} 个来源，综合期望 ${tuneAnalysis.totalExpectH.toFixed(1)}h/个 [数据来源: ${src}]`);
    } catch(e) { setStatus('分析失败: ' + e); }
  };

  // 切换数据来源提示
  $('tuneSourceHint').className = 'tune-source-hint';
  $('tuneSourceHint').textContent = '';

  // 计算推荐
  $('btnCalcRecommend').onclick = async () => {
    if (!tuneAnalysis) return setStatus('请先分析掉落来源');
    const itemName = tuneAnalysis.itemName;
    const targets = [];
    document.querySelectorAll('.tune-target-card').forEach(card => {
      const mapName = card.dataset.map;
      const hours = parseFloat(card.querySelector('input').value) || 0;
      if (hours > 0) {
        targets.push({ mapName, targetHours: hours });
      }
    });
    if (targets.length === 0) return setStatus('请至少设置一个地图的目标时间');
    try {
      const roundMode = $('tuneRoundMode').value || 'precision';
      tuneRecommend = await window.go.app.App.RecommendRates(itemName, targets, roundMode);
      renderTuneRecommend($('tuneRoundMode').selectedOptions[0].textContent);
      $('tuneRecommendSection').style.display = '';
      $('tuneCopySection').style.display = '';
      // 填充复制目标物品列表
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
    // 从表格读取用户可能修改过的值
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

function renderTuneSources(hasSim) {
  if (!tuneAnalysis) return;
  $('tuneSourceCount').textContent = `(${tuneAnalysis.sources.length}个来源)`;
  // 数据来源提示
  const hintEl = $('tuneSourceHint');
  if (hasSim) {
    hintEl.className = 'tune-source-hint sim';
    hintEl.textContent = '📊 数据来源：模拟结果（击杀数/掉落数/模拟时长）';
  } else {
    hintEl.className = 'tune-source-hint cfg';
    hintEl.textContent = '⚠️ 数据来源：爆率文件 + MonGen 刷新配置（建议先运行模拟获取更准确的数据）';
  }
  let html = `<table class="tune-table">
    <tr><th>地图</th><th>怪物</th><th>爆率</th><th>数量</th><th>每小时怪数</th><th>期望(小时/个)</th></tr>`;
  tuneAnalysis.sources.forEach(s => {
    const timeCls = s.expectHours <= 2 ? 'time-good' : s.expectHours <= 10 ? 'time-warn' : 'time-bad';
    html += `<tr>
      <td>${s.mapName}</td>
      <td>${s.monsterName}</td>
      <td class="num prob">${s.probStr}</td>
      <td class="num">${s.quantity}</td>
      <td class="num">${s.killPerHour.toFixed(1)}</td>
      <td class="num ${timeCls}">${s.expectHours.toFixed(1)}</td>
    </tr>`;
  });
  html += `<tr style="font-weight:600;background:var(--bg2)"><td colspan="4">综合</td><td class="num">${tuneAnalysis.totalKph.toFixed(1)}</td><td class="num">${tuneAnalysis.totalExpectH.toFixed(1)}</td></tr>`;
  html += '</table>';
  $('tuneSourceTable').innerHTML = html;
}

function renderTuneTargets() {
  if (!tuneAnalysis) return;
  // 按地图去重
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

function renderTuneRecommend(modeName) {
  if (!tuneRecommend) return;
  $('tuneRecommendCount').textContent = `(${tuneRecommend.length}条)`;
  let html = `<table class="tune-table">
    <tr><th>地图</th><th>怪物</th><th>当前爆率</th><th>推荐爆率</th><th>修改后期望</th><th>偏差</th></tr>`;
  tuneRecommend.forEach((c, i) => {
    const oldProb = c.oldNum + '/' + c.oldDen;
    const oldExpH = c.oldExpectH || 0;
    const dev = c.deviation || 0;
    const devSign = dev >= 0 ? '+' : '';
    const devCls = Math.abs(dev) < 5 ? 'dev-ok' : Math.abs(dev) < 15 ? 'dev-warn' : 'dev-bad';
    html += `<tr class="rec-row" data-old-exp="${oldExpH}" data-old-den="${c.oldDen}" data-target-h="${c.newExpectH / (1 + dev/100) || 0}">
      <td>${c.mapName}</td>
      <td>${c.monsterName}</td>
      <td class="num prob">${oldProb}</td>
      <td class="num">${c.newNum}/<input type="text" class="rec-den" value="${c.newDen}" data-idx="${i}" style="width:65px" /></td>
      <td class="num exp-h">${c.newExpectH.toFixed(1)}h</td>
      <td class="num ${devCls}">${devSign}${dev.toFixed(1)}%</td>
    </tr>`;
  });
  html += '</table>';
  $('tuneRecommendTable').innerHTML = html;

  // 实时更新：修改分母后自动重算期望小时和偏差
  document.querySelectorAll('.rec-den').forEach(input => {
    input.oninput = () => {
      const row = input.closest('.rec-row');
      const oldExpH = parseFloat(row.dataset.oldExp) || 0;
      const oldDen = parseInt(row.dataset.oldDen) || 1;
      const newDen = parseInt(input.value) || 1;
      const expCell = row.querySelector('.exp-h');
      const devCell = row.querySelector('td:last-child');
      const targetH = parseFloat(row.dataset.targetH) || 0;
      if (oldExpH > 0 && newDen > 0 && oldDen > 0) {
        const newExpectH = oldExpH * newDen / oldDen;
        expCell.textContent = newExpectH.toFixed(1) + 'h';
        // 计算偏差
        let dev = 0;
        if (targetH > 0) {
          dev = (newExpectH - targetH) / targetH * 100;
        }
        const devSign = dev >= 0 ? '+' : '';
        devCell.textContent = devSign + dev.toFixed(1) + '%';
        devCell.className = 'num ' + (Math.abs(dev) < 5 ? 'dev-ok' : Math.abs(dev) < 15 ? 'dev-warn' : 'dev-bad');
        const idx = parseInt(input.dataset.idx);
        if (!isNaN(idx) && tuneRecommend[idx]) {
          tuneRecommend[idx].newDen = newDen;
          tuneRecommend[idx].newExpectH = newExpectH;
          tuneRecommend[idx].deviation = dev;
        }
      }
    };
  });
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

// 物品列表刷新（加载爆率文件后调用）
async function refreshTuneItemList() {
  try {
    const allItems = await window.go.app.App.GetAllItemNames();
    const select = $('tuneItemSelect');
    select.innerHTML = '<option value="">-- 请选择物品 --</option>';
    (allItems || []).forEach(name => {
      const opt = document.createElement('option');
      opt.value = name;
      opt.textContent = name;
      select.appendChild(opt);
    });
    if (window._setAllTuneItems) window._setAllTuneItems(allItems || []);
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

// ===== 加载提示 =====
function showLoading(text) {
  $('loadingText').textContent = text || '加载中...';
  $('loadingOverlay').classList.add('show');
}
function hideLoading() {
  $('loadingOverlay').classList.remove('show');
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