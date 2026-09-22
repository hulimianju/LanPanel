'use strict';
'require view';
'require uci';

// LuCI 菜单入口：LanPanel 自带完整的网页界面，这里只提供跳转。
return view.extend({
	load: function() {
		return uci.load('lanpanel');
	},

	render: function() {
		var listen = uci.get('lanpanel', 'main', 'listen') || ':3080';
		var port = listen.split(':').pop() || '3080';
		var url = window.location.protocol + '//' + window.location.hostname + ':' + port + '/';

		return E('div', { 'class': 'cbi-map' }, [
			E('h2', {}, 'LanPanel'),
			E('div', { 'class': 'cbi-map-descr' },
				'局域网导航面板与设备发现。面板配置、设备列表与扫描设置都在 LanPanel 自己的页面中完成。'),
			E('div', { 'class': 'cbi-section' }, [
				E('p', {}, [ '访问地址：', E('code', {}, url) ]),
				E('p', {}, [
					E('a', { 'class': 'btn cbi-button cbi-button-apply', 'href': url, 'target': '_blank', 'rel': 'noopener' }, '打开 LanPanel')
				]),
				E('p', { 'style': 'color: #888' },
					'修改端口或数据目录：编辑 /etc/config/lanpanel 后执行 /etc/init.d/lanpanel restart')
			])
		]);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
