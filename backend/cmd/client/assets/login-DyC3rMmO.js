import{a as r,R as p,t as B,i as s,f as n,q as G,k as H,r as K,g as o,s as Q,u as X,h as Y}from"./index-CprlIZ3Y.js";import{E as Z}from"./ErrorWarning-D0-uG7Dr.js";var ee=B('<div class="min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8"><section class="max-w-md w-full space-y-8 bg-blue-950/50 backdrop-blur-sm p-8 rounded-xl shadow-lg shadow-blue-900/20"><header><h1 class="text-center text-3xl font-bold text-white">Sign in to your account</h1></header><form class="mt-8 space-y-6"aria-label="Login form"><fieldset class="space-y-4 rounded-md"><legend class=sr-only>Login credentials</legend><div><label for=username class=sr-only>Username</label><div class=relative><div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none"></div><input autofocus id=username name=username type=text required class="appearance-none relative block w-full px-10 py-2 border border-blue-800/20 bg-blue-900/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-300 focus:border-yellow-300 focus:z-10 sm:text-sm placeholder:text-blue-200/50"placeholder=Username aria-required=true></div></div><div><label for=email class=sr-only>Email</label><div class=relative><div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none"></div><input id=email name=email type=email required class="appearance-none relative block w-full px-10 py-2 border border-blue-800/20 bg-blue-900/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-300 focus:border-yellow-300 focus:z-10 sm:text-sm placeholder:text-blue-200/50"placeholder="Email address"aria-required=true></div></div><div><label for=password class=sr-only>Password</label><div class=relative><div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none"></div><input id=password name=password type=password required class="appearance-none relative block w-full px-10 py-2 border border-blue-800/20 bg-blue-900/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-300 focus:border-yellow-300 focus:z-10 sm:text-sm placeholder:text-blue-200/50"placeholder=Password aria-required=true></div></div></fieldset><div><button type=submit class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200">');const se=function(){const[m,q]=r(""),[g,P]=r(""),[v,T]=r(""),[j,d]=r(""),[u,h]=r(!1),[A,c]=r(!1),U=p.useNavigate(),z=p.useSearch(),F=async i=>{i.preventDefault(),h(!0),d(""),c(!1);try{const t=await fetch("/api/v1/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},credentials:"same-origin",body:JSON.stringify({username:m(),email:g(),password:v()})}),a=await t.json();if(!t.ok){d(a.error||t.statusText),c(!0);return}X({user:a.user,isAuthenticated:!0,isLoading:!1});const{redirect:l}=z();U({to:l,from:p.fullPath,replace:!0})}catch(t){console.error(t),d("A network error occurred"),c(!0)}finally{h(!1)}};return(()=>{var i=ee(),t=i.firstChild,a=t.firstChild,l=a.nextSibling,f=l.firstChild,R=f.firstChild,x=R.nextSibling,V=x.firstChild,I=V.nextSibling,w=I.firstChild,y=w.nextSibling,$=x.nextSibling,N=$.firstChild,O=N.nextSibling,_=O.firstChild,S=_.nextSibling,D=$.nextSibling,J=D.firstChild,M=J.nextSibling,C=M.firstChild,E=C.nextSibling,W=f.nextSibling,b=W.firstChild;return l.addEventListener("submit",F),s(l,n(Z,{get error(){return j()},get isVisible(){return A()}}),f),s(w,n(G,{class:"h-5 w-5 text-yellow-300","aria-hidden":"true"})),y.$$input=e=>q(e.currentTarget.value),s(_,n(H,{class:"h-5 w-5 text-yellow-300","aria-hidden":"true"})),S.$$input=e=>P(e.currentTarget.value),s(C,n(K,{class:"h-5 w-5 text-yellow-300","aria-hidden":"true"})),E.$$input=e=>T(e.currentTarget.value),s(b,()=>u()?"Signing in...":"Sign in"),o(e=>{var k=u(),L=u();return k!==e.e&&(b.disabled=e.e=k),L!==e.t&&Q(b,"aria-busy",e.t=L),e},{e:void 0,t:void 0}),o(()=>y.value=m()),o(()=>S.value=g()),o(()=>E.value=v()),i})()};Y(["input"]);export{se as component};
