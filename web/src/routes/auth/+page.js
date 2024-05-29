import { fail, redirect } from '@sveltejs/kit';

export const actions = {
  default: async ({ request, url }) => {
    const authData = await request.formData();

    const user = {
      username: formData.get('username'),
      email: formData.get('email'),
      password: formData.get('password')
    };

    const res = await fetch('/api/v1/auth', {
      method: 'post',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(user)
    });

    const r = await res.json();

    if (!res.ok) {
      return fail(res.status, r.error);
    }

    if (url.searchParams.has('redirectTo')) {
      redirect(303, url.searchParams.get('redirectTo'));
    }
  }
};
