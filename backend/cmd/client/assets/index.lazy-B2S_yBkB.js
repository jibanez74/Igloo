import{t as i,i as t,f as r,j as v,k as y,l as $,m as S,n as C,o as k,c as F,b as U,p as E,L as j,S as f}from"./index-CprlIZ3Y.js";import{S as L}from"./Spinner-Dkcz79uN.js";import{E as q}from"./ErrorWarning-D0-uG7Dr.js";var N=i('<table class=w-full role=grid><caption class=sr-only>List of system users with their details</caption><thead><tr class="text-left border-b border-blue-800/20"><th scope=col class="pb-3 text-sm font-medium text-white">Name</th><th scope=col class="pb-3 text-sm font-medium text-white">Username</th><th scope=col class="pb-3 text-sm font-medium text-white">Email</th><th scope=col class="pb-3 text-sm font-medium text-white">Status</th><th scope=col class="pb-3 text-sm font-medium text-white">Role</th></tr></thead><tbody class="divide-y divide-blue-800/20">'),R=i('<tr class="hover:bg-blue-800/20 transition-colors"><th scope=row class="py-4 text-white font-normal"></th><td class="py-4 text-blue-200"></td><td class="py-4 text-blue-200"><div class="flex items-center gap-2"></div></td><td class=py-4></td><td class=py-4>'),T=i('<span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-200"> Active'),z=i('<span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-red-400/10 text-red-400"> Inactive'),A=i('<span class="inline-flex items-center gap-1 text-yellow-300"> Admin'),M=i("<span class=text-blue-200>User");function P(a){return(()=>{var s=N(),n=s.firstChild,d=n.nextSibling,h=d.nextSibling;return t(h,r(v,{get each(){return a.users},children:e=>(()=>{var o=R(),m=o.firstChild,c=m.nextSibling,x=c.nextSibling,b=x.firstChild,g=x.nextSibling,w=g.nextSibling;return t(m,()=>e.name),t(c,()=>e.username),t(b,r(y,{class:"w-4 h-4","aria-hidden":!0}),null),t(b,()=>e.email,null),t(g,(()=>{var p=$(()=>!!e.is_active);return()=>p()?(()=>{var l=T(),u=l.firstChild;return t(l,r(S,{class:"w-3 h-3","aria-hidden":!0}),u),l})():(()=>{var l=z(),u=l.firstChild;return t(l,r(C,{class:"w-3 h-3","aria-hidden":!0}),u),l})()})()),t(w,(()=>{var p=$(()=>!!e.is_admin);return()=>p()?(()=>{var l=A(),u=l.firstChild;return t(l,r(k,{class:"w-4 h-4","aria-hidden":!0}),u),l})():M()})()),o})()})),s})()}var I=i('<p class="text-sm text-white">Total users: <span class="text-yellow-300 font-medium">'),K=i("<div class=overflow-x-auto>"),Q=i('<div class="container mx-auto"><section class="bg-blue-950/50 backdrop-blur-sm rounded-2xl p-6 shadow-lg shadow-blue-900/20 border border-blue-900/20"aria-label="Users list"><header class="mb-6 flex items-center justify-between"><div class="flex items-center gap-4"><h1 class="text-2xl font-bold text-yellow-300">Users'),_=i('<div class="h-40 flex items-center justify-center">'),V=i('<p class="h-40 flex items-center justify-center text-white"role=status>No users found');const G=F("/_auth/_admin/users/")({component:W});function W(){const a=U(()=>({queryKey:["users"],queryFn:async()=>{try{const s=await fetch("/api/v1/users",{credentials:"same-origin"}),n=await s.json();if(!s.ok)throw new Error(`${s.status} - ${n.error?n.error:s.statusText}`);return n}catch(s){throw console.error(s),new Error("a network error occurred while fetching users")}}}));return(()=>{var s=Q(),n=s.firstChild,d=n.firstChild,h=d.firstChild;return h.firstChild,t(h,r(j,{to:"/users/form",class:"inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-blue-950 bg-yellow-300 rounded-lg hover:bg-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors shadow-sm shadow-black/20",get children(){return[r(E,{class:"w-4 h-4","aria-hidden":"true"}),"New User"]}}),null),t(d,r(f,{get when(){return a.isSuccess},get children(){var e=I(),o=e.firstChild,m=o.nextSibling;return t(m,()=>{var c;return((c=a.data)==null?void 0:c.count)??0}),e}}),null),t(n,r(f,{get when(){return!a.isLoading},get fallback(){return(()=>{var e=_();return t(e,r(L,{size:"lg"})),e})()},get children(){return r(f,{get when(){return!a.isError},get fallback(){return(()=>{var e=_();return t(e,r(q,{get error(){var o;return((o=a.error)==null?void 0:o.message)??""},isVisible:!0})),e})()},get children(){return r(f,{get when(){var e;return(e=a.data)==null?void 0:e.items.length},get fallback(){return V()},get children(){var e=K();return t(e,r(P,{get users(){return a.data.items}})),e}})}})}}),null),s})()}export{G as Route};
