# Example artifacts

These static sites work offline. Publish a directory with the CLI or backoffice:

```sh
folio publish --title "Product pulse" ./examples/product-dashboard
folio publish --title "Team note" ./examples/team-note
folio publish --title "Launch checklist" ./examples/launch-checklist
```

Each site includes `index.html`. Product pulse has an interactive chart; team note uses relative JavaScript modules and CSS; launch checklist has interactive progress. All example numbers are sample data.
