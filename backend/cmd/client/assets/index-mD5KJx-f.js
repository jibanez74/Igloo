import{n as b,t as o,i as l,e as t,S as h,p as w,M as m,q as p,r as $,u as _}from"./index-Bu2rdP97.js";import{M as y}from"./MovieCard-7ubKMLKr.js";import{E as S}from"./ErrorWarning-c3OTVPay.js";import{S as C}from"./Spinner-CbMpCmfT.js";import"./getImgSrc-ClS6Ftii.js";var M=o('<div class="h-full flex items-center justify-center"><p class=text-blue-200/80>No movies available'),x=o('<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6">'),j=o('<section class=py-8><div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"><div class="flex items-center justify-between h-10 mb-6"><h2 class="text-2xl font-bold text-white"><span class="bg-gradient-to-r from-yellow-300 to-yellow-400 bg-clip-text text-transparent">Latest Movies</span></h2><div class="w-5 h-5 flex items-center justify-center"></div></div><div class=h-10></div><div class=min-h-[200px]>'),E=o('<div class="aspect-[2/3] rounded-xl bg-blue-950/50 animate-pulse">');function F(){const s=b(()=>({queryKey:["latest-movies"],queryFn:async()=>{try{const r=await fetch("/api/v1/movies/latest",{credentials:"include"}),i=await r.json();if(!r.ok)throw new Error(`${r.status} - ${i.error?i.error:r.statusText}`);return i}catch(r){throw console.error(r),new Error("a network error occurred while fetching latest movies")}}}));return(()=>{var r=j(),i=r.firstChild,c=i.firstChild,u=c.firstChild,d=u.nextSibling,n=c.nextSibling,g=n.nextSibling;return l(d,t(h,{get when(){return s.isPending},get children(){return t(C,{})}})),l(n,t(S,{get error(){var e;return((e=s.error)==null?void 0:e.message)||""},get isVisible(){return s.isError}})),l(g,t(h,{get when(){return!s.isPending},get fallback(){return(()=>{var e=x();return l(e,t(p,{get each(){return Array(12).fill(null)},children:()=>E()})),e})()},get children(){return t(w,{get children(){return[t(m,{get when(){var e;return((e=s.data)==null?void 0:e.movies.length)===0},get children(){return M()}}),t(m,{get when(){var e;return(e=s.data)==null?void 0:e.movies},get children(){var e=x();return l(e,t(p,{get each(){var a;return(a=s.data)==null?void 0:a.movies},children:a=>t(y,{movie:a,imgLoading:"eager"})})),e}})]}})}})),r})()}var q=o('<main class=min-h-screen><section class="relative h-[60vh] flex items-center justify-center overflow-hidden"><div class="absolute inset-0 bg-gradient-to-b from-blue-950/90 via-blue-950/80 to-blue-950"></div><div class="relative z-10 text-center px-4 sm:px-6 lg:px-8"><div class="max-w-3xl mx-auto space-y-6"><h1 class="text-4xl sm:text-5xl font-bold"><span class="bg-gradient-to-r from-yellow-300 to-yellow-400 bg-clip-text text-transparent">Welcome to Igloo</span></h1><p class="text-xl text-blue-200">Your personal media server for Movies, TV Shows, Music, and more</p><div class="flex justify-center gap-4"><button type=button class="inline-flex items-center gap-2 px-6 py-3 rounded-lg bg-blue-600 text-white hover:bg-blue-700 hover:shadow-lg hover:shadow-blue-600/20 transition-all duration-300"><span>Start Watching</span></button></div></div></div></section><div class=bg-transparent>');const V=function(){return(()=>{var r=q(),i=r.firstChild,c=i.firstChild,u=c.nextSibling,d=u.firstChild,n=d.firstChild,g=n.nextSibling,e=g.nextSibling,a=e.firstChild,v=a.firstChild,f=i.nextSibling;return l(d,t($,{class:"w-16 h-16 mx-auto text-yellow-300","aria-hidden":"true"}),n),l(a,t(_,{class:"w-5 h-5","aria-hidden":"true"}),v),l(f,t(F,{})),r})()};export{V as component};
