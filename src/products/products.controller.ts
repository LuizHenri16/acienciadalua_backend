import {
  Body,
  Controller,
  Delete,
  Get,
  Param,
  Patch,
  Post,
  Put,
  Query,
  UploadedFiles,
  UseGuards,
  UseInterceptors,
} from '@nestjs/common';
import { FileFieldsInterceptor } from '@nestjs/platform-express';
import { diskStorage } from 'multer';
import { extname } from 'path';
import { ApiBearerAuth, ApiBody, ApiConsumes, ApiOperation, ApiQuery, ApiTags } from '@nestjs/swagger';
import { Category } from '@prisma/client';
import { ProductsService } from './products.service';
import { CreateProductDTO } from './dtos/create-product.dto';
import { UpdateProductDTO } from './dtos/update-product.dto';
import { JwtAdminGuard } from '../jwt/jwt-admin.guard';

const storage = diskStorage({
  destination: './uploads',
  filename: (_, file, cb) => {
    const unique = `${Date.now()}-${Math.round(Math.random() * 1e9)}`;
    cb(null, `${unique}${extname(file.originalname)}`);
  },
});

@ApiTags('Products')
@ApiBearerAuth()
@UseGuards(JwtAdminGuard) // this guard ensures that the user is authenticated and is an admin
@Controller('products')
export class ProductsController {
  constructor(private readonly productsService: ProductsService) { }

  @Get()
  @ApiOperation({ summary: 'List all products' })
  @ApiQuery({ name: 'category', enum: Category, required: false })
  @ApiQuery({ name: 'isActive', type: Boolean, required: false })
  findAll(
    @Query('category') category?: Category,
    @Query('isActive') isActive?: string,
  ) {
    const parsedIsActive = isActive !== undefined ? isActive === 'true' : undefined;
    return this.productsService.findAll(category, parsedIsActive);
  }

  @Get(':id')
  @ApiOperation({ summary: 'Get product by id' })
  findOne(@Param('id') id: string) {
    return this.productsService.findOne(id);
  }

  @Post()
  @ApiOperation({ summary: 'Create a new product' })
  @ApiConsumes('multipart/form-data')
  @ApiBody({
    schema: {
      type: 'object',
      properties: {
        title: { type: 'string' },
        description: { type: 'string' },
        price: { type: 'number' },
        category: { type: 'string', enum: Object.values(Category) },
        cover: { type: 'string', format: 'binary' },
        file: { type: 'string', format: 'binary' },
      },
    },
  })
  @UseInterceptors(
    FileFieldsInterceptor([{ name: 'cover', maxCount: 1 }, { name: 'file', maxCount: 1 }], { storage }),
  )
  create(
    @Body() dto: CreateProductDTO,
    @UploadedFiles() files: { cover?: Express.Multer.File[]; file?: Express.Multer.File[] },
  ) {
    const coverUrl = files.cover?.[0]?.filename ?? '';
    const fileUrl = files.file?.[0]?.filename ?? '';
    return this.productsService.create(dto, coverUrl, fileUrl);
  }

  @Put(':id')
  @ApiOperation({ summary: 'Update a product' })
  update(@Param('id') id: string, @Body() dto: UpdateProductDTO) {
    return this.productsService.update(id, dto);
  }

  @Patch(':id/toggle')
  @ApiOperation({ summary: 'Toggle product isActive status' })
  toggle(@Param('id') id: string) {
    return this.productsService.toggle(id);
  }

  @Delete(':id')
  @ApiOperation({ summary: 'Delete a product' })
  remove(@Param('id') id: string) {
    return this.productsService.remove(id);
  }
}
