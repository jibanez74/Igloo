<script lang="ts">
    import type { PageProps } from './$types';

    let { data }: PageProps = $props();
</script>

<h1>Hello from Igloo</h1>

<section>
    <div>
        <h2>
            Latest Movies Go here
        </h2>
    </div>
</section>

<section>
    <div>
        <h2>Latest Albums</h2>

        <ul>
            {#each data.albums as album}
                <li>
                    {album.title} by {album.musician_name}
                </li>
                {/each}
        </ul>
    </div>
</section>