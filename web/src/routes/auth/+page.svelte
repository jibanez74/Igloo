<script>
  import { enhance } from '$app/forms';

  export let form;
</script>

<section title="authSection" class="h-screen flex justify-center items-center">
  <div class="rounded-lg p-5 bg-light text-dark">
    <h2>Sign In!</h2>

    <form method="POST" use:enhance>
      <div class="mb-2">
        <label for="username">Username</label>
        <input autofocus type="text" id="username" name="username" required minLength="2" />
      </div>

      <div class="mb-2">
        <label for="email">Email</label>
        <input type="email" id="email" name="email" required />
      </div>

      <div class="mb-2">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" required />
      </div>

      <button type="submit"> Submit </button>
    </form>
  </div>
</section>
