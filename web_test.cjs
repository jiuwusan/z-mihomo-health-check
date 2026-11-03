const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// Minimal DOM surface; the real app.js renders the response into these nodes.
class Element {
  constructor() {this.children = []; this.textContent = ''; this.value=''; this.listeners={}; this.classList = {toggle() {}};}
  addEventListener(name,fn) {this.listeners[name]=fn;}
  get options() {return this.children;}
  add(node) {this.children.push(node);}
  reportValidity() {return true;}
  removeAttribute(name) {delete this[name];}
  append(...nodes) {this.children.push(...nodes);}
  replaceChildren(...nodes) {this.children = nodes;}
}

test('a delayed pre-save status cannot replace newer status and history', async () => {
  const nodes = new Map();
  const pending = [];
  const context = vm.createContext({
    document: {
      getElementById(id) {if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id);},
      createElement() {return new Element();}
    },
    AbortSignal, Date, setTimeout() {},
    fetch(url) {
      // Leave initial configuration loading pending; exercise status requests directly.
      if (url === '/api/instances') return new Promise(() => {});
      assert.equal(url, '/api/instances/default/status');
      return new Promise(resolve => pending.push(resolve));
    }
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, 'web/app.js'), 'utf8'), context);
  vm.runInContext("selectedInstance='default'",context);
  const beforeSave = context.refreshPanel();
  const afterSave = context.refreshPanel();
  function status(historyID, current, message) {
    return {running:false, phase:'stopped', current, failures:0, pool_size:200, results:[],
      history_id:historyID, event_count:1,
      events:[{id:1, time:new Date().toISOString(), level:'info', message}]};
  }
  pending[1]({ok:true, json:async () => status('new-history', 'new-country', 'new-event')});
  await afterSave;
  pending[0]({ok:true, json:async () => status('old-history', 'old-country', 'old-event')});
  await beforeSave;
  assert.equal(nodes.get('current-group').textContent, 'new-country');
  const renderedMessage = nodes.get('event-list').children[0].children[2].textContent;
  assert.equal(renderedMessage, 'new-event');
});

test('a late configuration from another instance cannot overwrite the selected instance', async () => {
  const nodes=new Map(),pending=new Map();
  const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const context=vm.createContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,
    Option:class {constructor(text,value){this.textContent=text;this.value=value;}},
    fetch(url){
      if(url==='/api/instances')return new Promise(()=>{});
      if(url.endsWith('/status'))return Promise.resolve({ok:true,json:async()=>({phase:'stopped',events:[],results:[],history_id:url})});
      return new Promise(resolve=>pending.set(url,resolve));
    }});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  vm.runInContext("instanceList=[{id:'a',name:'A'},{id:'b',name:'B'}]",context);
  const first=context.switchInstance('a',true);
  const second=context.switchInstance('b',true);
  const cfg=url=>({api_url:url,group:'group',interval_seconds:5,failure_threshold:1,timeout_seconds:4,recovery_initial_delay_seconds:300,recovery_interval_seconds:60,recovery_success_threshold:3});
  pending.get('/api/instances/b/config')({ok:true,json:async()=>cfg('http://b:9090')});await second;
  pending.get('/api/instances/a/config')({ok:true,json:async()=>cfg('http://a:9090')});await first;
  assert.equal(get('api-url').value,'http://b:9090');
  assert.equal(get('instance-name').value,'B');
  assert.deepEqual(get('target-group').options.map(option=>option.value),['group']);
});

test('recovery controls serialize configuration and display progress', () => {
  const nodes = new Map();
  const get = id => {if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id);};
  const context = vm.createContext({document:{getElementById:get, createElement:() => new Element()},
    AbortSignal, Date, setTimeout() {}, fetch:() => new Promise(() => {})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  for (const [id,value] of Object.entries({'api-url':'http://mihomo:9090',secret:'','target-group':'🚀 节点选择',interval:'5',threshold:'1',timeout:'4','recovery-interval':'60','recovery-threshold':'3'})) get(id).value=value;
  get('recovery-enabled').checked=true;
  get('recovery-initial-delay').value='300';
  const payload=context.payload();
  assert.equal(payload.recovery_initial_delay_seconds,300);
  assert.equal(payload.recovery_enabled,true);
  assert.equal(payload.recovery_interval_seconds,60);
  assert.equal(payload.recovery_success_threshold,3);
  assert.equal('recovery_cooldown_seconds' in payload,false);
  context.renderStatus({phase:'healthy',running:true,results:[],recovery:{enabled:true,candidate:'香港',successes:2,threshold:3,message:'等待下一轮恢复检查'}});
  assert.match(get('recovery-status').textContent,/香港.*2\/3/);
  assert.doesNotMatch(get('recovery-status').textContent,/冷却/);
  context.renderStatus({phase:'healthy',running:true,results:[],recovery:{enabled:true,initial_wait_seconds:250}});
  assert.match(get('recovery-status').textContent,/首次检查.*250/);
  context.renderStatus({phase:'stopped',results:[],recovery:{enabled:false}});
  assert.equal(get('recovery-status').textContent,'自动恢复未运行');
});

test('the latest probe URL is visible and only HTTP links are clickable', () => {
  const nodes=new Map();const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const context=vm.createContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},fetch:()=>new Promise(()=>{})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  context.renderStatus({phase:'warning',results:[],last_site:'https://www.example.com/probe'});
  assert.equal(get('last-site').textContent,'https://www.example.com/probe');
  assert.equal(get('last-site').href,'https://www.example.com/probe');
  context.renderStatus({phase:'waiting',results:[],last_site:''});
  assert.equal(get('last-site').textContent,'尚未开始探查');
  assert.equal(get('last-site').href,undefined);
  context.renderStatus({phase:'warning',results:[],last_site:'javascript:alert(1)'});
  assert.equal(get('last-site').href,undefined);
});

test('late group, save and history responses cannot cross instance boundaries', async t => {
  for (const mode of ['groups','save','events']) await t.test(mode,async()=>{
    const nodes=new Map();const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
    let finish;
    const cfg=id=>({api_url:`http://${id}:9090`,group:`group-${id}`,interval_seconds:5,failure_threshold:1,timeout_seconds:4,recovery_initial_delay_seconds:300,recovery_interval_seconds:60,recovery_success_threshold:3});
    const context=vm.createContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,
      Option:class {constructor(text,value){this.textContent=text;this.value=value;}},
      fetch(url,options){
        if(url==='/api/instances')return new Promise(()=>{});
        if(url==='/api/instances/a/groups' || (url==='/api/instances/a/config' && options.method==='PUT') || url.includes('/a/events?'))return new Promise(resolve=>{finish=resolve;});
        const id=url.includes('/b/')?'b':'a';
        if(url.endsWith('/config'))return Promise.resolve({ok:true,json:async()=>cfg(id)});
        if(url.endsWith('/status'))return Promise.resolve({ok:true,json:async()=>({phase:'stopped',current:id,results:[],history_id:`history-${id}`,event_count:1,events:[{id:1,time:new Date().toISOString(),message:`event-${id}`} ]})});
        throw Error('unexpected '+url);
      }});
    vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
    vm.runInContext("instanceList=[{id:'a',name:'A'},{id:'b',name:'B'}]",context);
    await context.switchInstance('a',true);
    let pending;
    if(mode==='groups')pending=get('connect-button').listeners.click();
    if(mode==='save')pending=get('settings-form').listeners.submit({preventDefault(){}});
    if(mode==='events') {vm.runInContext('historyCursor=100',context);pending=context.refreshPanel();await new Promise(resolve=>setImmediate(resolve));}
    assert.equal(typeof finish,'function');
    await context.switchInstance('b',true);
    finish({ok:true,json:async()=>mode==='groups'?{groups:[{name:'old-a'}]}:mode==='save'?cfg('a'):{history_id:'history-a',events:[{id:1,time:new Date().toISOString(),message:'old-a'}],total:1}});
    await pending;
    assert.equal(get('api-url').value,'http://b:9090');
    assert.equal(get('target-group').value,'group-b');
    assert.deepEqual(get('target-group').options.map(option=>option.value),['group-b']);
    assert.equal(get('current-group').textContent,'b');
    assert.equal(get('event-list').children[0].children[2].textContent,'event-b');
  });
});

test('webhook fields load privately, serialize edits, and clear on instance switch', async () => {
  const nodes=new Map();
  const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const context=vm.createContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,Option:class {constructor(text,value){this.textContent=text;this.value=value;}},fetch:()=>new Promise(()=>{})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  context.applyConfig({api_url:'http://mihomo:9090',group:'test',dingtalk_webhook_set:true,feishu_webhook_set:false});
  assert.equal(get('dingtalk-webhook-state').textContent,'已保存 · 不回显');
  assert.equal(get('feishu-webhook-state').textContent,'未设置');
  get('dingtalk-webhook').value='https://example.com/ding';
  get('feishu-webhook').value='https://example.com/fei';
  get('clear-feishu-webhook').checked=true;
  const data=context.payload();
  assert.equal(data.dingtalk_webhook,'https://example.com/ding');
  assert.equal(data.feishu_webhook,'https://example.com/fei');
  assert.equal(data.clear_feishu_webhook,true);
  context.switchInstance('next',true);
  assert.equal(get('dingtalk-webhook').value,'');
  assert.equal(get('feishu-webhook').value,'');
  assert.equal(get('clear-feishu-webhook').checked,false);
});
