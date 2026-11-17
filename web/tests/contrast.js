// Measured rendered colors supplement axe's text checks with field, icon and
// focus visibility. This deliberately excludes uploaded artifact documents.
() => {
  const rgb = (v) => { const n=v.match(/[\d.]+/g)?.map(Number)||[0,0,0,0]; return [n[0],n[1],n[2],n[3]??1]; };
  const mix = (a,b) => a.slice(0,3).map((v,i)=>v*a[3]+b[i]*(1-a[3]));
  const bg = (el) => {
    const chain=[];for(let e=el;e;e=e.parentElement)chain.unshift(e);
    return chain.reduce((c,e)=>mix(rgb(getComputedStyle(e).backgroundColor),c),[255,255,255]);
  };
  const lum = c => c.slice(0,3).map(v=>{v/=255;return v<=0.04045?v/12.92:((v+0.055)/1.055)**2.4}).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
  const ratio = (a,b) => (Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05);
  const visible = e => { const r=e.getBoundingClientRect();return e.checkVisibility()&&r.width>0&&r.height>0&&r.right>0&&r.bottom>0&&r.left<innerWidth&&r.top<innerHeight&&!e.closest('[inert]'); };
  const scope=document.querySelector('[role="dialog"]')||document.body;
  const records=[];
  const add=(kind,e,value,required) => records.push({kind,target:e.getAttribute('aria-label')||e.getAttribute('placeholder')||e.className||e.tagName,ratio:+value.toFixed(4),required,pass:value>=required});
  for(const e of scope.querySelectorAll('input:not([type=checkbox]):not([type=radio]):not([type=hidden]):not([type=file]),textarea,select,[role="combobox"]')) {
    if(!visible(e)||e.disabled)continue;
    const s=getComputedStyle(e),inside=bg(e),outside=bg(e.parentElement);
    const boundary=Math.max(ratio(rgb(s.borderTopColor),inside),ratio(inside,outside));
    add('field-boundary',e,boundary,3);
    if(e.placeholder){const c=rgb(getComputedStyle(e,'::placeholder').color);add('placeholder',e,ratio(mix(c,inside),inside),4.5);}
  }
  for(const e of scope.querySelectorAll('.icon-button:not(:disabled)')) {
    if(!visible(e))continue;add('icon',e,ratio(rgb(getComputedStyle(e).color),bg(e)),3);
  }
  for(const e of scope.querySelectorAll('.meter i')) {
    if(visible(e))add('meter',e,ratio(bg(e),bg(e.parentElement)),3);
  }
  const active=document.activeElement;
  if(active&&scope.contains(active)&&active.matches(':focus-visible')){
    const s=getComputedStyle(active);
    if(s.outlineStyle!=='none'&&parseFloat(s.outlineWidth)>=2)add('keyboard-focus',active,ratio(rgb(s.outlineColor),bg(active.parentElement)),3);
    else records.push({kind:'keyboard-focus',target:active.tagName,ratio:0,required:3,pass:false});
  }
  return records;
}
