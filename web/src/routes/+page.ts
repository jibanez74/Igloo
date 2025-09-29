import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
    const [albumResult] = await Promise.all([
        fetch("http://localhost:8080/api/v1/albums/latest").then(res => res.json()),
    ])

    return {
        albums: albumResult.data.albums,
    }
}