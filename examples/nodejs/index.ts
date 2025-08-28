import * as pulumi from "@pulumi/pulumi";
import * as boilerplate from "@mynamespace/netcup";

const myRandomResource = new boilerplate.Random("myRandomResource", {
  length: 24,
});
const myRandomComponent = new boilerplate.RandomComponent("myRandomComponent", {
  length: 24,
});
export const output = {
  value: myRandomResource.result,
};
