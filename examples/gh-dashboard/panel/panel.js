(function () {
  var cfg = window.__diaPluginConfig || {};
  var repo = cfg.repo || '';
  var prLimit = cfg.pr_limit || 10;
  var issueLimit = cfg.issue_limit || 10;
  var actionLimit = cfg.action_limit || 5;

  var root = document.getElementById('root');
  if (!root) return;

  var tabs = [
    { id: 'prs', label: 'PRs', icon: '◇' },
    { id: 'issues', label: 'Issues', icon: '○' },
    { id: 'actions', label: 'Actions', icon: '▶' },
    { id: 'projects', label: 'Projects', icon: '☰' },
  ];

  var activeTab = 'prs';
  var tabData = {};
  var tabLoading = {};
  var tabError = {};

  function buildLayout() {
    root.innerHTML = '';
    root.style.cssText = 'display:flex;height:100%;font:14px -apple-system,Segoe UI,sans-serif;background:#1e1e2e;color:#cdd6f4;';

    var sidebar = document.createElement('nav');
    sidebar.style.cssText = 'width:180px;min-width:180px;background:#181825;border-right:1px solid #313244;display:flex;flex-direction:column;padding:8px 0;';

    var header = document.createElement('div');
    header.style.cssText = 'padding:8px 16px 12px;font-weight:700;font-size:13px;color:#89b4fa;border-bottom:1px solid #313244;margin-bottom:8px;';
    header.textContent = 'GitHub';
    sidebar.appendChild(header);

    for (var ti = 0; ti < tabs.length; ti++) {
      (function (tab) {
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.dataset.tab = tab.id;
        btn.style.cssText = [
          'display:flex', 'align-items:center', 'gap:8px', 'width:100%',
          'padding:10px 16px', 'border:0', 'background:transparent',
          'color:#a6adc8', 'cursor:pointer', 'font:13px -apple-system,Segoe UI,sans-serif',
          'text-align:left', 'transition:all 0.15s',
          'border-right:2px solid transparent',
        ].join(';');
        btn.innerHTML = '<span style="width:16px;text-align:center">' + tab.icon + '</span> ' + tab.label;
        btn.addEventListener('click', function () { activateTab(tab.id); });
        sidebar.appendChild(btn);
      })(tabs[ti]);
    }

    sidebar.appendChild(document.createElement('div'));

    var configBtn = document.createElement('button');
    configBtn.type = 'button';
    configBtn.textContent = '\u2699 config';
    configBtn.style.cssText = 'padding:10px 16px;border:0;background:transparent;color:#585b70;cursor:pointer;font:11px -apple-system,Segoe UI,sans-serif;text-align:left;border-top:1px solid #313244;margin-top:8px;';
    configBtn.addEventListener('click', function () {
      if (window.dia && window.dia.getConfig) {
        window.dia.getConfig().then(function (c) { alert(JSON.stringify(c, null, 2)); });
      }
    });
    sidebar.appendChild(configBtn);

    var content = document.createElement('main');
    content.style.cssText = 'flex:1;display:flex;flex-direction:column;overflow:hidden;min-width:0;';
    content.id = 'gh-content';

    root.appendChild(sidebar);
    root.appendChild(content);
  }

  function activateTab(id) {
    activeTab = id;
    var buttons = root.querySelectorAll('nav button[data-tab]');
    for (var i = 0; i < buttons.length; i++) {
      var b = buttons[i];
      if (b.dataset.tab === id) {
        b.style.background = '#1e1e2e';
        b.style.color = '#cdd6f4';
        b.style.borderRightColor = '#89b4fa';
      } else {
        b.style.background = 'transparent';
        b.style.color = '#a6adc8';
        b.style.borderRightColor = 'transparent';
      }
    }
    renderContent();
    if (!tabData[id]) fetchTabData(id);
  }

  function renderContent() {
    var content = document.getElementById('gh-content');
    if (!content) return;
    content.innerHTML = '';

    var tab = tabs.filter(function (t) { return t.id === activeTab; })[0];
    if (!tab) return;

    var hdr = document.createElement('div');
    hdr.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:12px 16px;border-bottom:1px solid #313244;';

    var title = document.createElement('h2');
    title.style.cssText = 'margin:0;font-size:15px;font-weight:600;color:#cdd6f4;';
    title.textContent = tab.label;
    hdr.appendChild(title);

    var actions = document.createElement('div');
    actions.style.cssText = 'display:flex;gap:6px;';

    var refreshBtn = document.createElement('button');
    refreshBtn.type = 'button';
    refreshBtn.textContent = 'refresh';
    refreshBtn.style.cssText = 'padding:4px 12px;background:#313244;color:#cdd6f4;border:0;border-radius:6px;cursor:pointer;font:12px -apple-system,Segoe UI,sans-serif;';
    refreshBtn.addEventListener('click', function () { fetchTabData(activeTab); });
    actions.appendChild(refreshBtn);

    var openBtn = document.createElement('button');
    openBtn.type = 'button';
    openBtn.textContent = 'open in browser';
    openBtn.style.cssText = 'padding:4px 12px;background:#313244;color:#cdd6f4;border:0;border-radius:6px;cursor:pointer;font:12px -apple-system,Segoe UI,sans-serif;';
    openBtn.addEventListener('click', function () { openTabInBrowser(activeTab); });
    actions.appendChild(openBtn);

    hdr.appendChild(actions);
    content.appendChild(hdr);

    var body = document.createElement('div');
    body.style.cssText = 'flex:1;overflow-y:auto;padding:8px 16px;';

    if (tabLoading[activeTab]) {
      body.innerHTML = '<div style="padding:40px;text-align:center;color:#585b70;font-size:13px;">Loading...</div>';
    } else if (tabError[activeTab]) {
      body.innerHTML = '<div style="padding:40px;text-align:center;color:#f38ba8;font-size:13px;">' + escapeHtml(tabError[activeTab]) + '</div>';
    } else if (tabData[activeTab]) {
      renderTabBody(activeTab, body);
    } else {
      body.innerHTML = '<div style="padding:40px;text-align:center;color:#585b70;font-size:13px;">Click refresh to load data.</div>';
    }

    content.appendChild(body);
  }

  function renderTabBody(tabId, body) {
    var data = tabData[tabId];
    if (!data || data.length === 0) {
      body.innerHTML = '<div style="padding:40px;text-align:center;color:#585b70;font-size:13px;">Nothing to show.</div>';
      return;
    }

    if (tabId === 'prs') renderPRs(body, data);
    else if (tabId === 'issues') renderIssues(body, data);
    else if (tabId === 'actions') renderActions(body, data);
    else if (tabId === 'projects') renderProjects(body, data);
  }

  function renderPRs(body, prs) {
    for (var i = 0; i < prs.length; i++) {
      var pr = prs[i];
      var row = createRow();
      var icon = document.createElement('span');
      icon.style.cssText = 'color:' + (pr.state === 'OPEN' ? '#a6e3a1' : pr.state === 'MERGED' ? '#89b4fa' : '#f38ba8') + ';font-size:14px;';
      icon.textContent = pr.state === 'OPEN' ? '◆' : pr.state === 'MERGED' ? '◆' : '◆';
      row.left.appendChild(icon);

      var info = document.createElement('div');
      info.style.cssText = 'flex:1;min-width:0;';
      var titleEl = document.createElement('div');
      titleEl.style.cssText = 'color:#cdd6f4;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
      titleEl.textContent = '#' + pr.number + '  ' + (pr.title || '(no title)');
      info.appendChild(titleEl);

      var meta = document.createElement('div');
      meta.style.cssText = 'color:#585b70;font-size:11px;margin-top:2px;';
      var branch = pr.headRefName || '';
      var author = (pr.author && pr.author.login) || 'unknown';
      meta.textContent = branch + ' by ' + author;
      info.appendChild(meta);

      row.left.appendChild(info);
      row.right.appendChild(createOpenBtn(pr.url));
      body.appendChild(row.el);
    }
  }

  function renderIssues(body, issues) {
    for (var i = 0; i < issues.length; i++) {
      var issue = issues[i];
      var row = createRow();
      var icon = document.createElement('span');
      icon.style.cssText = 'color:' + (issue.state === 'OPEN' ? '#a6e3a1' : '#585b70') + ';font-size:14px;';
      icon.textContent = issue.state === 'OPEN' ? '○' : '●';
      row.left.appendChild(icon);

      var info = document.createElement('div');
      info.style.cssText = 'flex:1;min-width:0;';
      var titleEl = document.createElement('div');
      titleEl.style.cssText = 'color:#cdd6f4;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
      titleEl.textContent = '#' + issue.number + '  ' + (issue.title || '(no title)');
      info.appendChild(titleEl);

      if (issue.labels && issue.labels.length > 0) {
        var labelsEl = document.createElement('div');
        labelsEl.style.cssText = 'display:flex;gap:4px;margin-top:3px;flex-wrap:wrap;';
        for (var li = 0; li < Math.min(issue.labels.length, 5); li++) {
          var lbl = issue.labels[li];
          var badge = document.createElement('span');
          badge.style.cssText = 'padding:1px 6px;border-radius:10px;font-size:10px;background:#313244;color:#a6adc8;';
          badge.textContent = lbl.name || '';
          labelsEl.appendChild(badge);
        }
        info.appendChild(labelsEl);
      }

      row.left.appendChild(info);
      row.right.appendChild(createOpenBtn(issue.url));
      body.appendChild(row.el);
    }
  }

  function renderActions(body, actions) {
    for (var i = 0; i < actions.length; i++) {
      var run = actions[i];
      var row = createRow();
      var icon = document.createElement('span');
      var statusColor = '#585b70';
      if (run.conclusion === 'success') statusColor = '#a6e3a1';
      else if (run.conclusion === 'failure') statusColor = '#f38ba8';
      else if (run.status === 'in_progress') statusColor = '#f9e2af';
      icon.style.cssText = 'color:' + statusColor + ';font-size:12px;';
      icon.textContent = run.status === 'in_progress' ? '▶' : run.conclusion === 'success' ? '✔' : run.conclusion === 'failure' ? '✘' : '●';
      row.left.appendChild(icon);

      var info = document.createElement('div');
      info.style.cssText = 'flex:1;min-width:0;';
      var nameEl = document.createElement('div');
      nameEl.style.cssText = 'color:#cdd6f4;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
      nameEl.textContent = run.name || '(no name)';
      info.appendChild(nameEl);

      var meta = document.createElement('div');
      meta.style.cssText = 'color:#585b70;font-size:11px;margin-top:2px;';
      meta.textContent = (run.headBranch || '') + ' - ' + (run.status === 'completed' ? run.conclusion : run.status);
      info.appendChild(meta);

      row.left.appendChild(info);
      var url = run.url || (run.htmlUrl ? run.htmlUrl : '');
      if (url) row.right.appendChild(createOpenBtn(url));
      body.appendChild(row.el);
    }
  }

  function renderProjects(body, projects) {
    for (var i = 0; i < projects.length; i++) {
      var proj = projects[i];
      var row = createRow();
      var icon = document.createElement('span');
      icon.style.cssText = 'color:#89b4fa;font-size:14px;';
      icon.textContent = '☰';
      row.left.appendChild(icon);

      var info = document.createElement('div');
      info.style.cssText = 'flex:1;min-width:0;';
      var nameEl = document.createElement('div');
      nameEl.style.cssText = 'color:#cdd6f4;font-size:13px;';
      nameEl.textContent = proj.title || proj.name || '(no title)';
      info.appendChild(nameEl);

      var meta = document.createElement('div');
      meta.style.cssText = 'color:#585b70;font-size:11px;margin-top:2px;';
      meta.textContent = '#' + (proj.number || '') + (proj.closed ? ' (closed)' : '');
      info.appendChild(meta);

      row.left.appendChild(info);
      row.right.appendChild(createOpenBtn(proj.url));
      body.appendChild(row.el);
    }
  }

  function createRow() {
    var el = document.createElement('div');
    el.style.cssText = 'display:flex;align-items:center;gap:10px;padding:8px 4px;border-bottom:1px solid #313244;cursor:default;';
    el.addEventListener('mouseenter', function () { el.style.background = '#181825'; });
    el.addEventListener('mouseleave', function () { el.style.background = 'transparent'; });

    var left = document.createElement('div');
    left.style.cssText = 'display:flex;align-items:center;gap:8px;flex:1;min-width:0;';

    var right = document.createElement('div');
    right.style.cssText = 'display:flex;align-items:center;gap:4px;';

    el.appendChild(left);
    el.appendChild(right);
    return { el: el, left: left, right: right };
  }

  function createOpenBtn(url) {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = '\u2197';
    btn.title = 'open in browser';
    btn.style.cssText = 'padding:2px 6px;background:#313244;color:#a6adc8;border:0;border-radius:4px;cursor:pointer;font:12px sans-serif;';
    btn.addEventListener('click', function (e) {
      e.stopPropagation();
      if (window.dia && window.dia.call) {
        window.dia.call('exec', ['gh', url ? url.split('/').slice(-2).join('/') : '']).catch(function () {});
        window.open(url, '_blank');
      } else {
        window.open(url, '_blank');
      }
    });
    return btn;
  }

  function openTabInBrowser(tabId) {
    var baseUrl = 'https://github.com';
    var path = repo ? '/' + repo : '';
    switch (tabId) {
      case 'prs': path += '/pulls'; break;
      case 'issues': path += '/issues'; break;
      case 'actions': path += '/actions'; break;
      case 'projects': path += '/projects'; break;
    }
    window.open(baseUrl + path, '_blank');
  }

  function fetchTabData(tabId) {
    tabLoading[tabId] = true;
    tabError[tabId] = null;
    renderContent();

    var cmd;
    switch (tabId) {
      case 'prs':
        cmd = ['gh', 'pr', 'list', '--json', 'number,title,headRefName,state,url,author', '--limit', String(prLimit)];
        if (repo) { cmd.push('--repo', repo); }
        break;
      case 'issues':
        cmd = ['gh', 'issue', 'list', '--json', 'number,title,state,url,labels', '--limit', String(issueLimit)];
        if (repo) { cmd.push('--repo', repo); }
        break;
      case 'actions':
        cmd = ['gh', 'run', 'list', '--json', 'name,status,conclusion,headBranch,url,createdAt,number', '--limit', String(actionLimit)];
        if (repo) { cmd.push('--repo', repo); }
        break;
      case 'projects':
        cmd = ['gh', 'project', 'list', '--json', 'title,number,url,closed', '--limit', '20'];
        if (repo) { cmd = ['gh', 'project', 'list', '--repo', repo, '--json', 'title,number,url,closed', '--limit', '20']; }
        break;
    }

    if (!cmd) return;

    window.dia.call('exec', cmd).then(function (raw) {
      tabLoading[tabId] = false;
      if (!raw) { tabData[tabId] = []; renderContent(); return; }
      try {
        tabData[tabId] = JSON.parse(raw);
      } catch (e) {
        tabError[tabId] = 'Failed to parse output: ' + e.message;
      }
      renderContent();
    }).catch(function (err) {
      tabLoading[tabId] = false;
      tabError[tabId] = String(err);
      renderContent();
    });
  }

  function escapeHtml(s) {
    if (!s) return '';
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  buildLayout();
  activateTab('prs');
})();
