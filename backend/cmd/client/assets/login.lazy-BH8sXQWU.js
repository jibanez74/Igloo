import{c as w,r as t,u as y,a as v,j as e,E as N,F as S,b as E,d as F}from"./index-DTInXHC2.js";const q=w("/login")({component:L});function L(){const[n,c]=t.useState(!1),[l,r]=t.useState(""),[x,u]=t.useState(!1),{setUser:f,user:d}=y(),m=v();t.useEffect(()=>{d&&m({to:"/",replace:!0})},[d,m]),t.useEffect(()=>{if(l){u(!0);const a=setTimeout(()=>{u(!1),setTimeout(()=>r(""),300)},6e3);return()=>clearTimeout(a)}},[l]);const p=async a=>{a.preventDefault(),r(""),c(!0);const i=new FormData(a.currentTarget),b=i.get("username"),h=i.get("email"),g=i.get("password");try{const s=await fetch("/api/v1/auth/login",{method:"POST",credentials:"include",headers:{"Content-Type":"application/json"},body:JSON.stringify({username:b,email:h,password:g})});if(!s.ok){const o=await s.json().catch(()=>null);throw new Error((o==null?void 0:o.error)||"Something went wrong. Please try again later.")}const j=await s.json();f(j.user)}catch(s){s instanceof Error?r(s.message):r("Something went wrong. Please try again later.")}finally{c(!1)}};return e.jsx("main",{className:"min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8",children:e.jsxs("section",{className:"max-w-md w-full space-y-8 bg-slate-900/50 backdrop-blur-sm p-8 rounded-xl shadow-lg shadow-blue-900/20",children:[e.jsx("header",{children:e.jsx("h1",{className:"text-center text-3xl font-bold text-white",children:"Sign in to your account"})}),e.jsxs("form",{className:"mt-8 space-y-6",onSubmit:p,"aria-label":"Login form",children:[e.jsx(N,{error:l,isVisible:x}),e.jsxs("fieldset",{className:"space-y-4 rounded-md",children:[e.jsx("legend",{className:"sr-only",children:"Login credentials"}),e.jsxs("div",{children:[e.jsx("label",{htmlFor:"username",className:"sr-only",children:"Username"}),e.jsxs("div",{className:"relative",children:[e.jsx("div",{className:"absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none",children:e.jsx(S,{className:"h-5 w-5 text-blue-200","aria-hidden":"true"})}),e.jsx("input",{autoFocus:!0,id:"username",name:"username",type:"text",required:!0,className:"appearance-none relative block w-full px-10 py-2 border border-blue-900/20 bg-slate-800/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm placeholder:text-slate-400",placeholder:"Username","aria-required":"true"})]})]}),e.jsxs("div",{children:[e.jsx("label",{htmlFor:"email",className:"sr-only",children:"Email"}),e.jsxs("div",{className:"relative",children:[e.jsx("div",{className:"absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none",children:e.jsx(E,{className:"h-5 w-5 text-blue-200","aria-hidden":"true"})}),e.jsx("input",{id:"email",name:"email",type:"email",required:!0,className:"appearance-none relative block w-full px-10 py-2 border border-blue-900/20 bg-slate-800/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm placeholder:text-slate-400",placeholder:"Email address","aria-required":"true"})]})]}),e.jsxs("div",{children:[e.jsx("label",{htmlFor:"password",className:"sr-only",children:"Password"}),e.jsxs("div",{className:"relative",children:[e.jsx("div",{className:"absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none",children:e.jsx(F,{className:"h-5 w-5 text-blue-200","aria-hidden":"true"})}),e.jsx("input",{id:"password",name:"password",type:"password",required:!0,className:"appearance-none relative block w-full px-10 py-2 border border-blue-900/20 bg-slate-800/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm placeholder:text-slate-400",placeholder:"Password","aria-required":"true"})]})]})]}),e.jsx("div",{children:e.jsx("button",{type:"submit",disabled:n,className:"group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200","aria-busy":n,children:n?"Signing in...":"Sign in"})})]})]})})}export{q as Route};
