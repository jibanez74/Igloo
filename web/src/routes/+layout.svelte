<script lang="ts">
    import '../app.css';
    import favicon from '$lib/assets/favicon.svg';

    let {children} = $props();
</script>

<svelte:head>
    <link rel="icon" href={favicon}/>
</svelte:head>

<header>
    <nav>
        navbar goes here
    </nav>
</header>

<main>
    {@render children?.()}
</main>

<footer>
    <div>
        <h6>
            footer goes here
        </h6>
    </div>
</footer>