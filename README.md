# Confuzzled

Procedural Puzzle Generator and Multiplayer Game made using a Go backend API and Typescript (HTML Canvas) frontend.

## Usage

The backend in the `server` directory hosts both the API and the compiled static site files. You can compile a binary using `go build` or simply run the backend in place using `go run`. If you choose the latter, all user configuration or build artifacts files have already been added to the `.gitignore` at the root of this directory (i.e., this should not make your repo instance dirty according to Git).

Follow both of these steps to run this web app:

### Setup Backend

1. Use either the `server` directory in this repo or create a folder that contains the backend's executable binary file.
2. In this folder, supply `image.png`
    - This image should preferably be high-resolution
    - Keep the aspect ratio within reason
    - The image must be in `png` format and must be named `image.png`
3. Optionally, also create `config.json`. The defaults are shown below. You can use this as a template for your own configuration file.

    ```json
    {
        "puzzleRows": 5,
        "puzzleCols": 5,
        "puzzleSize": 0.7,
        "margin": 1.3,
        "perturb": 0.3,
        "perturbFrequency": 0.02
    }
    ```

4. Create a `serve` folder in the `server` directory.

### Setup Frontend

1. Run `npm run build` in the `web` directory of this repo.
2. Copy everything in `web/dist` (not the directory itself, just everything inside of it) into the `serve` folder we created in Step 4 in "Setup Backend". 

### Run Backend

Run the backend by using `go run .` while inside of the `server` directory. If you chose to compile a binary for the backend, run that instead.

### Run Frontend

If you copied the output of `npm run build` in the `dist` directory to `server/serve`, the frontend is also hosted along with the API on the same port `:8080`. Open up `localhost:8080` to see the website in action.

### Development

Alternatively, if you want to develop the website while the API is running in the background, you must delete the `serve` directory and instead concurrently host the API and backend. To prevent CORS errors, Vite has been configured to also run an API proxy on `:8080`. This can be done by running `npm run dev` in the `web` directory along with `go run .` in the `server` directory concurrently.

## Technical Details

This codebase is composed of a simple frontend Typescript application (uses plain CSS and HTML Canvas) and a backend that generates and hosts an API for each puzzle.

### Procedural Generation

The most challening part of making this program lies in `server/generate.go`. Given the configuration the user provides, we have to figure out a way to split the image into the given amount of puzzle rows and columns. We also have to generate the tabs (the parts of puzzle pieces that stick out) and blanks (sockets).

Here is a chronological overview of what the program does to accomplish this:

1. Load puzzle input image
2. Downsize the image such that there is no remainder for `puzzleCols` (width) or `puzzleRows` (height)
    - This may involve a slight change in the aspect ratio of the image (this is extremely small, normally we only have to drop a few pixels)
3. Define an index format for each individual puzzle piece
    - Rather than making a 2D slice for each piece's data, a 1D flat map with IDs referring to two dimensions was easier to work with
    - There should be `puzzleCols * puzzleRows` pieces
    - In our case, we share an ID format of `r${row}c${column}` (i.e., `r3c2` for Row 3 Column 2) for the server and client
4. Allocate all of the pieces in the map and give them an appropriate key using the aforementioned ID function
5. Traverse each piece in the map and assign each of the four sides tabs or blanks. A connection must contain a tab on one piece and a blank on the other.
6. Split the downscaled puzzle image (covers the whole puzzle) into individual segments that represent each piece.
    - Since puzzle pieces also contain features that extend beyond these simple rectangular sections, we sample a larger part of the main image and make it transparent.
    - These transparent margins will later contain perturbation (waves) and tabs/blanks.
    - We use NRGBA to preserve color data while our alpha channel is modulated.
7. Apply 1D noise perturbation to all 4 sides of each piece (adds waves to the each side of the shape)
    - We use a sine wave with the period, amplitude, and x-shift controlled by randomized coefficients
    - This is what `perturb` and `perturbFrequency` refer to in `config.json`
    - This may result in pixels in the margin being flipped on (the piece grows larger than its original rectangular domain) and pixels inside of the original rectangular section being made transparent (smaller)
8. Now draw tabs and blanks for each piece. This was the hardest step to implement.
    - One of my goals in this project was to make everything procedural. As a result, I didn't elect to use predefined masks for each puzzle to accomplish this.
    - We essentially need to generate a 2D array of (x, y) points where each successive point has an equally-spaced x and y represents the top half of one puzzle piece tab.
    - Mathematically, the end of a puzzle piece tab can be modelled by a semicircle. The rest of the tab (the "neck" of the tab nearer to the piece itself) can be modelled by a line tangent to the circle at point *c*. Point *c* is controlled by a randomized coefficient for each piece connection (a pair of a tab and a consecutive piece's blank). The tangent line should have a slope that causes the line to reach *y = b* at *x = 0* or at (*b*, 0), where *b* is another randomly set coefficient.
    - The code below accomplishes this:
        ```go
        p.curve.FitWithDerivatives(
            []float64{0, c},
            []float64{b, semicircle(c)},
            []float64{0, semicircleDerivative(c)},
        )
        ```
    - This effectively generates a continous curve that we can sample from.
    - We look "inside" of this curve relative to the image's margin for each side and flip the transparency on.
    - For each blank, we invert this and look outside of the generated curve.
    - Each connection pair stores a pointer to a `TabGenerator` struct that also has a struct method to get any x-value within the interval. We do this to preserve each connection's random coefficients (i.e., no two connection pairs are the same).
9. Export each piece as `{ID}.png` in the `out/img/` directory, with the ID exactly corresponding to our aforementioned ID function.

### API Routes

The API currently implemented is enough for all core functionality. If this were a production application, I would implement data validation and authentication on the server side. Currently, there is nothing preventing a single rogue user from ruining the current instance of the puzzle at hand. However, I was careful of preventing any XSS vectors and making sure the API runs fast enough (each request only takes a few ms).

#### POST /api/users/
