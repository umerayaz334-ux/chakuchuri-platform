import { DirectoryBrowsePanel } from "./trade-directory";

export function publicDirectoryFromPath(pathname: string) {
  const clean = `/${pathname}`.replace(/\/{2,}/g, "/").replace(/\/$/, "").toLowerCase();
  return clean === "/directory" || clean === "/public/directory";
}

export function PublicDirectoryPage() {
  return (
    <main className="cc-public-directory">
      <header>
        <div>
          <span className="cc-kicker">Wazirabad</span>
          <h1>Trade directory</h1>
          <p>Browse verified shops and makers. Listings appear here after ChakuChuri review.</p>
        </div>
        <a className="cc-button" href="/">
          Sign in
        </a>
      </header>
      <DirectoryBrowsePanel publicMode />
    </main>
  );
}
