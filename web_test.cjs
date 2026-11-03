const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function appContext(options) {
  return vm.createContext({AbortController, clearTimeout() {}, ...options, document:{hidden:false, addEventListener() {}, ...options.document}});
}

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
  showModal() {this.open=true;}
  close() {this.open=false;}
  scrollIntoView() {this.scrolled=true;}
}

test('a delayed pre-save status cannot replace newer status and history', async () => {
  const nodes = new Map();
  const pending = [];
  const context = appContext({
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
  const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,
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
  const context = appContext({document:{getElementById:get, createElement:() => new Element()},
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
  const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},fetch:()=>new Promise(()=>{})});
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
    const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,
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
  const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,Option:class {constructor(text,value){this.textContent=text;this.value=value;}},fetch:()=>new Promise(()=>{})});
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

test('group search retains original priority and can locate current selection', () => {
  const nodes=new Map();const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},fetch:()=>new Promise(()=>{})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  context.renderStatus({phase:'healthy',current:'日本',results:[{name:'美国',available:false},{name:'日本',available:true,delay_ms:90},{name:'新加坡',available:true,delay_ms:150}]});
  assert.match(get('group-summary').textContent,/2.*3/);
  get('group-search').value='日本';get('group-search').listeners.input();
  assert.equal(get('priority-list').children.length,1);
  assert.equal(get('priority-list').children[0].children[0].textContent,'02');
  get('group-search').value='不存在';get('group-search').listeners.input();
  assert.match(get('priority-list').children[0].textContent,/没有匹配/);
  get('locate-current').listeners.click();
  assert.equal(get('group-search').value,'');
  assert.equal(get('priority-list').children.length,3);
  assert.equal(get('priority-list').children[1].scrolled,true);
  context.renderStatus({phase:'waiting',results:[]});
  assert.equal(get('locate-current').disabled,true);
});

test('configuration dialog preserves an unsaved draft when closed and reopened', () => {
  const nodes=new Map();const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},fetch:()=>new Promise(()=>{})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  vm.runInContext('loaded=true',context);
  get('open-settings').listeners.click();
  assert.equal(get('settings-dialog').open,true);
  get('api-url').value='http://draft:9090';get('settings-form').listeners.input();
  get('close-settings').listeners.click();
  assert.equal(get('settings-dialog').open,false);
  get('open-settings').listeners.click();
  assert.equal(get('api-url').value,'http://draft:9090');
  assert.match(get('open-settings').textContent,/未保存/);
});

test('instance configuration load errors remain visible outside closed dialogs', async () => {
  const nodes=new Map();const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const context=appContext({document:{getElementById:get,createElement:()=>new Element()},AbortSignal,Date,setTimeout(){},confirm:()=>true,
    fetch:url=>url==='/api/instances'?new Promise(()=>{}):Promise.resolve({ok:false,json:async()=>({error:'无法读取实例配置'})})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  await context.switchInstance('broken',true);
  assert.match(get('panel-message').textContent,/无法读取实例配置/);
});

function pollingHarness() {
  const nodes=new Map(),timers=new Map(),requests=[],listeners={};let timerID=0;
  const get=id=>{if(!nodes.has(id))nodes.set(id,new Element());return nodes.get(id);};
  const status=id=>({phase:'healthy',current:id,events:[],results:[],history_id:id});
  const cfg=id=>({api_url:`http://${id}:9090`,group:id,interval_seconds:5,failure_threshold:1,timeout_seconds:4});
  let handle=(url)=>Promise.resolve({ok:true,json:async()=>url.endsWith('/config')?cfg(url):status(url)});
  const context=appContext({document:{getElementById:get,createElement:()=>new Element(),addEventListener:(type,fn)=>listeners[type]=fn},AbortSignal,Date,
    Option:class {constructor(text,value){this.textContent=text;this.value=value;}},confirm:()=>true,
    setTimeout(fn,ms){const id=++timerID;timers.set(id,{fn,ms});return id;},clearTimeout(id){timers.delete(id);},
    fetch(url,options){if(url==='/api/instances')return new Promise(()=>{});requests.push({url,options});return handle(url,options);}});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  vm.runInContext("selectedInstance='a';loaded=true",context);
  return {context,get,timers,requests,listeners,setHandle(fn){handle=fn;},async tick(){assert.equal(timers.size,1);const [id,timer]=[...timers][0];timers.delete(id);await timer.fn();}};
}
const settle=()=>new Promise(resolve=>setImmediate(resolve));

test('hidden pages pause polling and visible pages immediately resume a single loop', async () => {
  const h=pollingHarness();
  await h.context.poll();assert.equal(h.requests.length,1);assert.equal([...h.timers.values()][0].ms,2000);
  h.context.document.hidden=true;h.listeners.visibilitychange();assert.equal(h.timers.size,0);
  await h.context.poll();assert.equal(h.requests.length,1);
  h.context.document.hidden=false;h.listeners.visibilitychange();await settle();
  assert.equal(h.requests.length,2);assert.equal(h.timers.size,1);
  h.listeners.visibilitychange();await settle();assert.equal(h.timers.size,1);
});

test('poll retry delay backs off to 30 seconds and resets after success', async () => {
  const h=pollingHarness();h.setHandle(()=>Promise.reject(Error('offline')));
  await h.context.poll();
  for(const delay of [4000,8000,16000,30000,30000]) {
    assert.equal(h.timers.size,1);assert.equal([...h.timers.values()][0].ms,delay);await h.tick();
  }
  h.setHandle(()=>Promise.resolve({ok:true,json:async()=>({phase:'healthy',current:'recovered',events:[],results:[],history_id:'a'})}));
  await h.tick();assert.equal([...h.timers.values()][0].ms,2000);
  assert.equal(h.get('current-group').textContent,'recovered');
});

test('hiding aborts an in-flight poll without a connection error or stale rendering', async () => {
  const h=pollingHarness();let finish;
  h.setHandle(()=>new Promise(resolve=>finish=resolve));
  const pending=h.context.poll();const signal=h.requests[0].options.signal;
  h.context.document.hidden=true;h.listeners.visibilitychange();assert.equal(signal.aborted,true);
  finish({ok:true,json:async()=>({phase:'healthy',current:'stale',events:[],results:[]})});await pending;
  assert.notEqual(h.get('current-group').textContent,'stale');assert.notEqual(h.get('monitor-badge').textContent,'面板连接中断');assert.equal(h.timers.size,0);
});

test('switching instances aborts outstanding requests and the old poll cannot schedule retries', async () => {
  const h=pollingHarness();let rejectPoll;
  h.setHandle((url,options)=>{
    if(url.includes('/a/'))return new Promise((resolve,reject)=>{if(url.endsWith('/status'))rejectPoll=reject;});
    return Promise.resolve({ok:true,json:async()=>url.endsWith('/config')?{api_url:'http://b:9090',group:'b',interval_seconds:5,failure_threshold:1,timeout_seconds:4}:{phase:'healthy',current:'b',events:[],results:[],history_id:'b'}});
  });
  const pending=h.context.poll();
  // Exercise the same scoped request path used by connection tests, saves and history.
  h.context.instanceAPI('/api/groups',{signal:AbortSignal.timeout(10000)});
  h.context.instanceAPI('/api/config',{method:'PUT'});
  h.context.instanceAPI('/api/events?before=1');
  const old=h.requests.slice();assert.equal(old.length,4);
  await h.context.switchInstance('b',true);
  assert.ok(old.every(request=>request.options.signal.aborted));
  rejectPoll(Error('late old failure'));await pending;
  assert.equal(h.get('current-group').textContent,'b');assert.equal(h.timers.size,1);assert.equal([...h.timers.values()][0].ms,2000);
});

test('a new visible poll can finish before the canceled hidden request settles', async () => {
  const h=pollingHarness();let finishOld;let calls=0;
  h.setHandle(()=>++calls===1?new Promise(resolve=>finishOld=resolve):Promise.resolve({ok:true,json:async()=>({phase:'healthy',current:'fresh',events:[],results:[],history_id:'fresh'})}));
  const pending=h.context.poll();
  await h.context.poll();assert.equal(h.requests.length,1);
  h.context.document.hidden=true;h.listeners.visibilitychange();
  h.context.document.hidden=false;h.listeners.visibilitychange();await settle();
  assert.equal(h.get('current-group').textContent,'fresh');assert.equal(h.timers.size,1);
  finishOld({ok:true,json:async()=>({phase:'healthy',current:'old',events:[],results:[],history_id:'old'})});await pending;
  assert.equal(h.get('current-group').textContent,'fresh');assert.equal(h.timers.size,1);
});

test('configuration loaded while hidden waits until visible to request status', async () => {
  const h=pollingHarness();h.context.document.hidden=true;
  await h.context.switchInstance('b',true);
  assert.deepEqual(h.requests.map(request=>request.url),['/api/instances/b/config']);assert.equal(h.timers.size,0);
  h.context.document.hidden=false;h.listeners.visibilitychange();await settle();
  assert.equal(h.requests[1].url,'/api/instances/b/status');assert.equal(h.timers.size,1);
});

test('switching during configuration load cancels that request without surfacing an error', async () => {
  const h=pollingHarness();let rejectOld;
  h.setHandle((url)=>url==='/api/instances/old/config'?new Promise((resolve,reject)=>rejectOld=reject):Promise.resolve({ok:true,json:async()=>url.endsWith('/config')?{api_url:'http://b:9090',group:'b',interval_seconds:5,failure_threshold:1,timeout_seconds:4}:{phase:'healthy',current:'b',events:[],results:[],history_id:'b'}}));
  const pending=h.context.switchInstance('old',true);const signal=h.requests[0].options.signal;
  await h.context.switchInstance('b',true);assert.equal(signal.aborted,true);
  rejectOld(Error('aborted old load'));await pending;
  assert.equal(h.get('panel-message').textContent,'');assert.equal(h.get('current-group').textContent,'b');assert.equal(h.timers.size,1);
});

test('a hanging automatic poll retains the default timeout and enters backoff', async () => {
  const h=pollingHarness(),timeouts=[];
  h.context.AbortSignal={any:signals=>AbortSignal.any(signals),timeout(ms){const controller=new AbortController();timeouts.push({ms,controller});return controller.signal;}};
  h.setHandle((url,options)=>new Promise((resolve,reject)=>options.signal.addEventListener('abort',()=>reject(Error('timeout')),{once:true})));
  const pending=h.context.poll();
  assert.equal(timeouts.length,1);assert.equal(timeouts[0].ms,6000);
  timeouts[0].controller.abort();await pending;
  assert.equal(h.get('monitor-badge').textContent,'面板连接中断');assert.equal([...h.timers.values()][0].ms,4000);
});

test('connection tests can extend their timeout while remaining cancelable on switch', async () => {
  const h=pollingHarness(),timeouts=[];
  h.context.AbortSignal={any:signals=>AbortSignal.any(signals),timeout(ms){timeouts.push(ms);return new AbortController().signal;}};
  h.setHandle(()=>Promise.resolve({ok:true,json:async()=>({groups:[]})}));
  h.get('timeout').value='32';
  await h.get('connect-button').listeners.click();
  assert.equal(timeouts[0],36000);
});
